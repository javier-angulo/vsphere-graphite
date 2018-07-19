package backend

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"net/http"

	"github.com/cblomart/vsphere-graphite/backend/thininfluxclient"
	influxclient "github.com/influxdata/influxdb/client/v2"
	"github.com/marpaia/graphite-golang"
	"github.com/olivere/elastic"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Backend Interface
type Backend interface {
	Init(config Config) error
	Disconnect()
	SendMetrics(metrics []*Point)
}

// MapStr represents a json node (string reference to an object)
type MapStr map[string]interface{}

const (
	// Graphite name of the graphite backend
	Graphite = "graphite"
	// InfluxDB name of the influx db backend
	InfluxDB = "influxdb"
	// ThinInfluxDB name of the thin influx db backend
	ThinInfluxDB = "thininfluxdb"
	// InfluxTag is the tag for influxdb
	InfluxTag = "influx"
	// Elastic name of the elastic backend
	Elastic = "elastic"
	// Prometheus name of the prometheus backend
	Prometheus = "prometheus"
)

var stdlog, errlog *log.Logger

// Init : initialize a backend
func (backend *Config) Init(standardLogs *log.Logger, errorLogs *log.Logger) error {
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
		thininfluxclt, err := thininfluxclient.NewThinInlfuxClient(backend.Hostname, backend.Port, backend.Database, backend.Username, backend.Password, "s", backend.Encrypted)
		if err != nil {
			errlog.Println("Error creating thin InfluxDB client")
			return err
		}
		backend.thininfluxdb = &thininfluxclt
		return nil
	case Elastic:
		//Initialize Elastic client
		elasticindex := backend.Database
		if len(elasticindex) > 0 {
			elasticindex = elasticindex + "-" + time.Now().Format("2006.01.02")
		} else {
			errlog.Println("backend.Database (used as Elastic Index name) not specified in vsphere-graphite.json")
		}
		stdlog.Println("Initializing " + backendType + " backend " + backend.Hostname + ":" + strconv.Itoa(backend.Port) + "/" + elasticindex)
		protocol := "http"
		if backend.Encrypted {
			protocol = "https"
		}
		elasticclt, err := elastic.NewClient(
			elastic.SetURL(protocol+"://"+backend.Hostname+":"+strconv.Itoa(backend.Port)),
			elastic.SetMaxRetries(10),
			elastic.SetScheme(protocol),
			elastic.SetBasicAuth(backend.Username, backend.Password))
		if err != nil {
			errlog.Println("Error creating Elastic client")
			return err
		}
		backend.elastic = elasticclt
		return CreateIndexIfNotExists(backend.elastic, elasticindex)
	case Prometheus:
		//Initialize Prometheus client
		stdlog.Println("Initializing " + backendType + " backend")

		registry := prometheus.NewRegistry()
		err := registry.Register(backend)
		if err != nil {
			errlog.Println("Error creating Prometheus Registry")
			return err
		}

		http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError}))
		go func() error {
			err := http.ListenAndServe(":9155", nil)
			if err != nil {
				errlog.Println("Error creating Prometheus listener")
				return err
			}
			return nil
		}()
		return nil
	default:
		errlog.Println("Backend " + backendType + " unknown.")
		return errors.New("Backend " + backendType + " unknown.")
	}
}

// Disconnect : disconnect from backend
func (backend *Config) Disconnect() {
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
	case Elastic:
		// Disconnect from Elastic
		errlog.Println("Disconnecting from elastic")
	case Prometheus:
		// Stop Exporting Prometheus Metrics
		errlog.Println("Stopping exporter")
	default:
		errlog.Println("Backend " + backendType + " unknown.")
	}
}

// SendMetrics : send metrics to backend
func (backend *Config) SendMetrics(metrics []*Point) {
	switch backendType := strings.ToLower(backend.Type); backendType {
	case Graphite:
		var graphiteMetrics []graphite.Metric
		for _, point := range metrics {
			if point == nil {
				continue
			}
			//key := "vsphere." + vcName + "." + entityName + "." + name + "." + metricName
			key := fmt.Sprintf("vsphere.%s.%s.%s.%s.%s.%s", point.VCenter, point.ObjectType, point.ObjectName, point.Group, point.Counter, point.Rollup)
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
			if point == nil {
				continue
			}
			key := fmt.Sprintf("%s_%s_%s", point.Group, point.Counter, point.Rollup)
			tags := point.GetTags(backend.NoArray, "\\,")
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
			if point == nil {
				continue
			}
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
	case Elastic:
		elasticindex := backend.Database + "-" + time.Now().Format("2006.01.02")
		err := CreateIndexIfNotExists(backend.elastic, elasticindex)
		if err != nil {
			errlog.Println(err)
		} else {
			bulkRequest := backend.elastic.Bulk()
			for _, point := range metrics {
				indexReq := elastic.NewBulkIndexRequest().Index(elasticindex).Type("doc").Doc(point).UseEasyJSON(true)
				bulkRequest = bulkRequest.Add(indexReq)
			}
			bulkResponse, err := bulkRequest.Do(context.Background())
			if err != nil {
				// Handle error
				errlog.Println(err)
			} else {
				// Succeeded actions
				succeeded := bulkResponse.Succeeded()
				stdlog.Println("Logs successfully indexed: ", len(succeeded))
				_, err = backend.elastic.Flush().Index(elasticindex).Do(context.Background())
				if err != nil {
					panic(err)
				} else {
					stdlog.Println("Elastic Indexing flushed")
				}
			}
		}
	default:
		errlog.Println("Backend " + backendType + " unknown.")
	}
}
