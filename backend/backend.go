package backend

import (
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	influxclient "github.com/influxdata/influxdb/client/v2"
	"github.com/marpaia/graphite-golang"
)

// Point : Information collected for a point
type Point struct {
	VCenter      string
	ObjectType   string
	ObjectName   string
	Group        string
	Counter      string
	Instance     string
	Rollup       string
	Value        int64
	Datastore    []string
	ESXi         string
	Cluster      string
	Network      []string
	ResourcePool string
	Folder       string
	Timestamp    int64
}

// Backend : storage backend
type Backend struct {
	Hostname   string
	Port       int
	Database   string
	Username   string
	Password   string
	Type       string
	NoArray    bool
	carbon     *graphite.Graphite
	influx     influxclient.Client
	ValueField string
}

var stdlog, errlog *log.Logger
var carbon graphite.Graphite

// Init : initialize a backend
func (backend *Backend) Init(standardLogs *log.Logger, errorLogs *log.Logger) error {
	stdlog := standardLogs
	errlog := errorLogs
	if len(backend.ValueField) == 0 {
		// for compatibility reason with previous version
		// can now be changed in the config file.
		// the default can later be changed to another value.
		// most probably "value" (lower case)
		backend.ValueField = "Value"
	}
	switch backendType := strings.ToLower(backend.Type); backendType {
	case "graphite":
		// Initialize Graphite
		stdlog.Println("Intializing " + backendType + " backend")
		carbon, err := graphite.NewGraphite(backend.Hostname, backend.Port)
		if err != nil {
			errlog.Println("Error connecting to graphite")
			return err
		}
		backend.carbon = carbon
		return nil
	case "influxdb":
		//Initialize Influx DB
		stdlog.Println("Intializing " + backendType + " backend")
		influxclt, err := influxclient.NewHTTPClient(influxclient.HTTPConfig{
			Addr:     "http://" + backend.Hostname + ":" + strconv.Itoa(backend.Port),
			Username: backend.Username,
			Password: backend.Password,
		})
		if err != nil {
			errlog.Println("Error connecting to InfluxDB")
			return err
		}
		backend.influx = influxclt
		return nil
	default:
		errlog.Println("Backend " + backendType + " unknown.")
		return errors.New("Backend " + backendType + " unknown.")
	}
}

// Disconnect : disconnect from backend
func (backend *Backend) Disconnect() {
	switch backendType := strings.ToLower(backend.Type); backendType {
	case "graphite":
		// Disconnect from graphite
		stdlog.Println("Disconnecting from graphite")
		backend.carbon.Disconnect()
	case "influxdb":
		// Disconnect from influxdb
		stdlog.Println("Disconnecting from influxdb")
		_, _, err := backend.influx.Ping(3 * time.Second)
		if err == nil {
			backend.influx.Close()
		}
	default:
		errlog.Println("Backend " + backendType + " unknown.")
	}
}

// SendMetrics : send metrics to backend
func (backend *Backend) SendMetrics(metrics []Point) {
	switch backendType := strings.ToLower(backend.Type); backendType {
	case "graphite":
		var graphiteMetrics []graphite.Metric
		for _, point := range metrics {
			//key := "vsphere." + vcName + "." + entityName + "." + name + "." + metricName
			key := "vsphere." + point.VCenter + "." + point.ObjectType + "." + point.ObjectName + "." + point.Group + "." + point.Counter + "." + point.Rollup
			if len(point.Instance) > 0 {
				key += "." + strings.ToLower(strings.Replace(point.Instance, ".", "_", -1))
			}
			graphiteMetrics = append(graphiteMetrics, graphite.Metric{Name: key, Value: strconv.FormatInt(point.Value, 10), Timestamp: point.Timestamp})
		}
		err := backend.carbon.SendMetrics(graphiteMetrics)
		if err != nil {
			errlog.Println("Error sending metrics (trying to reconnect): ", err)
			backend.carbon.Connect()
		}
	case "influxdb":
		//Influx batch points
		bp, err := influxclient.NewBatchPoints(influxclient.BatchPointsConfig{
			Database:  backend.Database,
			Precision: "s",
		})
		if err != nil {
			errlog.Println("Error creating influx batchpoint")
			errlog.Println(err)
			return
		}
		for _, point := range metrics {
			key := point.Group + "_" + point.Counter + "_" + point.Rollup
			tags := map[string]string{}
			tags["vcenter"] = point.VCenter
			tags["type"] = point.ObjectType
			tags["name"] = point.ObjectName
			if backend.NoArray {
				if len(point.Datastore) > 0 {
					tags["datastore"] = point.Datastore[0]
				} else {
					tags["datastore"] = ""
				}
			} else {
				tags["datastore"] = strings.Join(point.Datastore, "\\,")
			}
			if backend.NoArray {
				if len(point.Network) > 0 {
					tags["network"] = point.Network[0]
				} else {
					tags["network"] = ""
				}
			} else {
				tags["network"] = strings.Join(point.Network, "\\,")
			}
			tags["host"] = point.ESXi
			tags["cluster"] = point.Cluster
			tags["instance"] = point.Instance
			tags["resourcepool"] = point.ResourcePool
			fields := make(map[string]interface{})
			fields[backend.ValueField] = point.Value
			pt, err := influxclient.NewPoint(key, tags, fields, time.Unix(point.Timestamp, 0))
			if err != nil {
				errlog.Println("Could not create influxdb point")
				errlog.Println(err)
				continue
			}
			bp.AddPoint(pt)
		}
		err = backend.influx.Write(bp)
		if err != nil {
			errlog.Println("Error sending metrics: ", err)
		}
	default:
		errlog.Println("Backend " + backendType + " unknown.")
	}
}
