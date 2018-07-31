package backend

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/cblomart/vsphere-graphite/utils"

	"net/http"

	"github.com/cblomart/vsphere-graphite/backend/thininfluxclient"
	"github.com/fluent/fluent-logger-golang/fluent"
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
	// Fluentd name of the fluentd backend
	Fluentd = "fluentd"
	// ThinPrometheus name of the thin prometheus backend
	ThinPrometheus = "thinprometheus"
)

var (
	query, done *chan bool
	metrics     *chan Point
	prefix      string
)

// Init : initialize a backend
func (backend *Config) Init() error {
	prefix = backend.Prefix
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
		log.Println("Intializing " + backendType + " backend")
		carbon, err := graphite.NewGraphite(backend.Hostname, backend.Port)
		if err != nil {
			log.Println("Error connecting to graphite")
			return err
		}
		backend.carbon = carbon
		return nil
	case InfluxDB:
		//Initialize Influx DB
		log.Println("Intializing " + backendType + " backend")
		protocol := "http"
		if backend.Encrypted {
			protocol = "https"
		}
		influxclt, err := influxclient.NewHTTPClient(influxclient.HTTPConfig{
			Addr:     fmt.Sprintf("%s://%s:%d", protocol, backend.Hostname, backend.Port),
			Username: backend.Username,
			Password: backend.Password,
		})
		if err != nil {
			log.Println("Error connecting to InfluxDB")
			return err
		}
		backend.influx = &influxclt
		return nil
	case ThinInfluxDB:
		//Initialize thin Influx DB client
		log.Println("Initializing " + backendType + " backend")
		thininfluxclt, err := thininfluxclient.NewThinInlfuxClient(backend.Hostname, backend.Port, backend.Database, backend.Username, backend.Password, "s", backend.Encrypted)
		if err != nil {
			log.Println("Error creating thin InfluxDB client")
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
			log.Println("backend.Database (used as Elastic Index name) not specified in vsphere-graphite.json")
		}
		log.Println("Initializing " + backendType + " backend " + backend.Hostname + ":" + strconv.Itoa(backend.Port) + "/" + elasticindex)
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
			log.Println("Error creating Elastic client")
			return err
		}
		backend.elastic = elasticclt
		return CreateIndexIfNotExists(backend.elastic, elasticindex)
	case Prometheus:
		//Initialize Prometheus client
		log.Println("Initializing " + backendType + " backend")

		registry := prometheus.NewRegistry()
		err := registry.Register(backend)
		if err != nil {
			log.Println("Error creating Prometheus Registry")
			return err
		}

		http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError}))
		go func() error {
			address := ""
			if len(backend.Hostname) > 0 {
				address = backend.Hostname
			}
			if backend.Port > 0 {
				address += ":" + utils.ValToString(backend.Port, "", false)
			} else {
				address += ":9155"
			}
			err := http.ListenAndServe(address, nil)
			if err != nil {
				log.Println("Error creating Prometheus listener")
				return err
			}
			log.Printf("Prometheus lisenting at http://%s/metrics\n", address)
			return nil
		}()
		return nil
	case Fluentd:
		//Initialize Influx DB
		log.Println("Initializing " + backendType + " backend")
		fluentclt, err := fluent.New(fluent.Config{FluentPort: backend.Port, FluentHost: backend.Hostname, MarshalAsJSON: true})
		if err != nil {
			log.Println("Error connecting to Fluentd")
			return err
		}
		backend.fluent = fluentclt
		return nil
	case ThinPrometheus:
		//Initialize Thin Prometheus
		log.Printf("Initializing %s backend\n", backendType)
		client, err := NewThinPrometheusClient(backend.Hostname, backend.Port)
		if err != nil {
			log.Println("Error connecting to Thin prometheus")
			return err
		}
		go func() {
			err := client.ListenAndServe()
			if err != nil {
				log.Printf("Error Starting Prometheus listener: %s", err)
			}
		}()
		return nil
	default:
		log.Println("Backend " + backendType + " unknown.")
		return errors.New("Backend " + backendType + " unknown.")
	}
}

// Disconnect : disconnect from backend
func (backend *Config) Disconnect() {
	switch backendType := strings.ToLower(backend.Type); backendType {
	case Graphite:
		// Disconnect from graphite
		log.Println("Disconnecting from graphite")
		err := backend.carbon.Disconnect()
		if err != nil {
			log.Println("Error disconnecting from graphite: ", err)
		}
	case InfluxDB:
		// Disconnect from influxdb
		log.Println("Disconnecting from influxdb")
	case ThinInfluxDB:
		// Disconnect from thin influx db
		log.Println("Disconnecting from thininfluxdb")
	case Elastic:
		// Disconnect from Elastic
		log.Println("Disconnecting from elastic")
	case Prometheus:
		// Stop Exporting Prometheus Metrics
		log.Println("Stopping Prometheus exporter")
	case Fluentd:
		// Disconnect from Elastic
		log.Println("Disconnecting from fluent")
	case ThinPrometheus:
		// Stop exporting Prometheus metrics
		log.Println("Disconnect ThinPrometheus exporter")
	default:
		log.Println("Backend " + backendType + " unknown.")
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
			key := fmt.Sprintf("%s.%s.%s.%s.%s.%s.%s", backend.Prefix, point.VCenter, point.ObjectType, point.ObjectName, point.Group, point.Counter, point.Rollup)
			if len(point.Instance) > 0 {
				key += "." + strings.ToLower(strings.Replace(point.Instance, ".", "_", -1))
			}
			graphiteMetrics = append(graphiteMetrics, graphite.Metric{Name: key, Value: strconv.FormatInt(point.Value, 10), Timestamp: point.Timestamp})
		}
		err := backend.carbon.SendMetrics(graphiteMetrics)
		if err != nil {
			log.Println("Error sending metrics (trying to reconnect): ", err)
			err := backend.carbon.Connect()
			if err != nil {
				log.Println("could not connect to graphite: ", err)
			}
		}
	case InfluxDB:
		//Influx batch points
		bp, err := influxclient.NewBatchPoints(influxclient.BatchPointsConfig{
			Database:  backend.Database,
			Precision: "s",
		})
		if err != nil {
			log.Println("Error creating influx batchpoint")
			log.Println(err)
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
				log.Println("Could not create influxdb point")
				log.Println(err)
				continue
			}
			bp.AddPoint(pt)
		}
		err = (*backend.influx).Write(bp)
		if err != nil {
			log.Println("Error sending metrics: ", err)
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
				log.Println("Error sending metrics: ", err)
				if err.Error() == "Server Busy: timeout" {
					log.Println("waiting .5 second to continue")
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
			log.Println("Error sendg metrics: ", err)
		}
	case Elastic:
		elasticindex := backend.Database + "-" + time.Now().Format("2006.01.02")
		err := CreateIndexIfNotExists(backend.elastic, elasticindex)
		if err != nil {
			log.Println(err)
		} else {
			bulkRequest := backend.elastic.Bulk()
			for _, point := range metrics {
				indexReq := elastic.NewBulkIndexRequest().Index(elasticindex).Type("doc").Doc(point).UseEasyJSON(true)
				bulkRequest = bulkRequest.Add(indexReq)
			}
			bulkResponse, err := bulkRequest.Do(context.Background())
			if err != nil {
				// Handle error
				log.Println(err)
			} else {
				// Succeeded actions
				succeeded := bulkResponse.Succeeded()
				log.Println("Logs successfully indexed: ", len(succeeded))
				_, err = backend.elastic.Flush().Index(elasticindex).Do(context.Background())
				if err != nil {
					panic(err)
				} else {
					log.Println("Elastic Indexing flushed")
				}
			}
		}
	case Prometheus:
		// Prometheus doesn't need to send metrics
	case Fluentd:
		for _, point := range metrics {
			if point != nil {
				err := backend.fluent.Post(backend.Prefix, *point)
				if err != nil {
					log.Println(err)
				}
			}
		}
	case ThinPrometheus:
		// Thin Prometheus doesn't need to send metrics
	default:
		log.Println("Backend " + backendType + " unknown.")
	}
}

// InitChannels initiates channels for pull backends
func (backend *Config) InitChannels(initquery *chan bool, initdone *chan bool, initmetrics *chan Point) {
	done = initdone
	query = initquery
	metrics = initmetrics
}

// Scheduled indicates that the metric collection needs to be scheduled for the backend
func (backend *Config) Scheduled() bool {
	switch backendType := strings.ToLower(backend.Type); backendType {
	case Prometheus:
		return false
	case ThinPrometheus:
		return false
	default:
		return true
	}
}
