package backend

import (
	"bytes"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/cblomart/vsphere-graphite/backend/ThinInfluxClient"
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
	ViTags       []string
	NumCPU       int32
	MemorySizeMB int32
	Timestamp    int64
}

// Backend : storage backend
type Backend struct {
	Hostname     string
	ValueField   string
	Database     string
	Username     string
	Password     string
	Type         string
	Port         int
	NoArray      bool
	Encrypted    bool
	carbon       *graphite.Graphite
	influx       *influxclient.Client
	thininfluxdb *ThinInfluxClient.ThinInfluxClient
}

const (
	// Graphite name of the graphite backend
	Graphite = "graphite"
	// InfluxDB name of the influx db backend
	InfluxDB = "influxdb"
	// ThinInfluxDB name of the thin influx db backend
	ThinInfluxDB = "thininfluxdb"
)

var stdlog, errlog *log.Logger
var carbon graphite.Graphite

// ToInflux serialises the data to be consumed by influx line protocol
// see https://docs.influxdata.com/influxdb/v1.2/write_protocols/line_protocol_tutorial/
func (point *Point) ToInflux(noarray bool, valuefield string) string {
	// buffer containing the resulting line
	buff := bytes.NewBuffer(nil)
	// measurement name
	buff.WriteString(point.Group)
	buff.WriteString("_")
	buff.WriteString(point.Counter)
	buff.WriteString("_")
	buff.WriteString(point.Rollup)
	// tags name=value
	buff.WriteString(",vcenter=")
	buff.WriteString(point.VCenter)
	buff.WriteString(",type=")
	buff.WriteString(point.ObjectType)
	buff.WriteString(",name=")
	buff.WriteString(point.ObjectName)
	// these value could have multiple values
	datastore := ""
	network := ""
	vitags := ""
	if noarray {
		if len(point.Datastore) > 0 {
			datastore = point.Datastore[0]
		}
		if len(point.Network) > 0 {
			network = point.Network[0]
		}
		if len(point.ViTags) > 0 {
			vitags = point.ViTags[0]
		}
	} else {
		if len(point.Datastore) > 0 {
			datastore = strings.Join(point.Datastore, "\\,")
		}
		if len(point.Network) > 0 {
			network = strings.Join(point.Network, "\\,")
		}
		if len(point.ViTags) > 0 {
			vitags = strings.Join(point.ViTags, "\\,")
		}
	}
	if len(datastore) > 0 {
		buff.WriteString(",datastore=")
		buff.WriteString(datastore)
	}
	if len(network) > 0 {
		buff.WriteString(",network=")
		buff.WriteString(network)
	}
	if len(vitags) > 0 {
		buff.WriteString(",vitags=")
		buff.WriteString(vitags)
	}
	if len(point.ESXi) > 0 {
		buff.WriteString(",host=")
		buff.WriteString(point.ESXi)
	}
	if len(point.Cluster) > 0 {
		buff.WriteString(",cluster=")
		buff.WriteString(point.Cluster)
	}
	if len(point.Instance) > 0 {
		buff.WriteString(",instance=")
		buff.WriteString(point.Instance)
	}
	if len(point.ResourcePool) > 0 {
		buff.WriteString(",resourcepool=")
		buff.WriteString(point.ResourcePool)
	}
	if len(point.Folder) > 0 {
		buff.WriteString(",folder=")
		buff.WriteString(point.Folder)
	}
	buff.WriteString(" ")
	buff.WriteString(valuefield)
	buff.WriteString("=")
	buff.WriteString(strconv.FormatInt(point.Value, 10))
	buff.WriteString("i")
	buff.WriteString(" ")
	buff.WriteString(strconv.FormatInt(point.Timestamp, 10))
	return buff.String()
}

// Init : initialize a backend
func (backend *Backend) Init(standardLogs *log.Logger, errorLogs *log.Logger) error {
	stdlog = standardLogs
	errlog = errorLogs
	if len(backend.ValueField) == 0 {
		// for compatibility reason with previous version
		// can now be changed in the config file.
		// the default can later be changed to another value.
		// most probably "value" (lower case)
		backend.ValueField = "Value"
	}
	switch backendType := strings.ToLower(backend.Type); backendType {
	case Graphite:
		// Initialize Graphite
		stdlog.Println("Intializing " + backendType + " backend")
		carbon, err := graphite.NewGraphite(backend.Hostname, backend.Port)
		if err != nil {
			errlog.Println("Error connecting to graphite")
			return err
		}
		backend.carbon = carbon
		return nil
	case InfluxDB:
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
		backend.influx = &influxclt
		return nil
	case ThinInfluxDB:
		//Initialize thin Influx DB client
		stdlog.Println("Initializing " + backendType + " backend")
		thininfluxclt, err := ThinInfluxClient.NewThinInlfuxClient(backend.Hostname, backend.Port, backend.Database, backend.Username, backend.Password, "s", backend.Encrypted)
		if err != nil {
			errlog.Println("Error creating thin InfluxDB client")
			return err
		}
		backend.thininfluxdb = &thininfluxclt
		return nil
	default:
		errlog.Println("Backend " + backendType + " unknown.")
		return errors.New("Backend " + backendType + " unknown.")
	}
}

// Disconnect : disconnect from backend
func (backend *Backend) Disconnect() {
	switch backendType := strings.ToLower(backend.Type); backendType {
	case Graphite:
		// Disconnect from graphite
		stdlog.Println("Disconnecting from graphite")
		err := backend.carbon.Disconnect()
		if err != nil {
			errlog.Println("Error disconnecting from graphite: ", err)
		}
	case InfluxDB:
		// Disconnect from influxdb
		stdlog.Println("Disconnecting from influxdb")
	case ThinInfluxDB:
		// Disconnect from thin influx db
		errlog.Println("Disconnecting from thininfluxdb")
	default:
		errlog.Println("Backend " + backendType + " unknown.")
	}
}

// SendMetrics : send metrics to backend
func (backend *Backend) SendMetrics(metrics []Point) {
	switch backendType := strings.ToLower(backend.Type); backendType {
	case Graphite:
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
			err := backend.carbon.Connect()
			if err != nil {
				errlog.Println("could not connect to graphite: ", err)
			}
		}
	case InfluxDB:
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
				}
			} else {
				if len(point.Datastore) > 0 {
					tags["datastore"] = strings.Join(point.Datastore, "\\,")
				}
			}
			if backend.NoArray {
				if len(point.Network) > 0 {
					tags["network"] = point.Network[0]
				}
			} else {
				if len(point.Network) > 0 {
					tags["network"] = strings.Join(point.Network, "\\,")
				}
			}
			if len(point.ESXi) > 0 {
				tags["host"] = point.ESXi
			}
			if len(point.Cluster) > 0 {
				tags["cluster"] = point.Cluster
			}
			if len(point.Instance) > 0 {
				tags["instance"] = point.Instance
			}
			if len(point.ResourcePool) > 0 {
				tags["resourcepool"] = point.ResourcePool
			}
			if len(point.Folder) > 0 {
				tags["folder"] = point.Folder
			}
			if backend.NoArray {
				if len(point.ViTags) > 0 {
					tags["vitags"] = point.ViTags[0]
				}
			} else {
				if len(point.ViTags) > 0 {
					tags["vitags"] = strings.Join(point.ViTags, "\\,")
				}
			}
			if point.NumCPU != 0 {
				tags["numcpu"] = strconv.FormatInt(int64(point.NumCPU), 10)
			}
			if point.MemorySizeMB != 0 {
				tags["memorysizemb"] = strconv.FormatInt(int64(point.MemorySizeMB), 10)
			}
			fields := make(map[string]interface{})
			fields[backend.ValueField] = point.Value
			pt, err := influxclient.NewPoint(key, tags, fields, time.Unix(point.Timestamp, 0)) // nolint: vetshadow
			if err != nil {
				errlog.Println("Could not create influxdb point")
				errlog.Println(err)
				continue
			}
			bp.AddPoint(pt)
		}
		err = (*backend.influx).Write(bp)
		if err != nil {
			errlog.Println("Error sending metrics: ", err)
		}
	case ThinInfluxDB:
		lines := []string{}
		for _, point := range metrics {
			lines = append(lines, point.ToInflux(backend.NoArray, backend.ValueField))
		}
		count := 3
		for count > 0 {
			err := backend.thininfluxdb.Send(lines)
			if err != nil {
				errlog.Println("Error sending metrics: ", err)
				if err.Error() == "Server Busy: timeout" {
					errlog.Println("waiting .5 second to continue")
					time.Sleep(500 * time.Millisecond)
					count--
				} else {
					break
				}
			} else {
				break
			}
		}
		err := backend.thininfluxdb.Send(lines)
		if err != nil {
			errlog.Println("Error sendg metrics: ", err)
		}
	default:
		errlog.Println("Backend " + backendType + " unknown.")
	}
}
