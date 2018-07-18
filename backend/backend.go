package backend

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"

	"encoding/json"
	"net/http"

	"github.com/cblomart/vsphere-graphite/backend/thininfluxclient"
	"github.com/cblomart/vsphere-graphite/utils"
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

// GetInfluxPoint : convert a point to an influxpoint
func (p *Point) GetInfluxPoint(noarray bool, valuefield string) *InfluxPoint {
	keyParts := make(map[int]string)
	ip := InfluxPoint{
		Fields: make(map[string]string),
		Tags:   make(map[string]string),
	}
	v := reflect.ValueOf(p).Elem()
	for i := 0; i < v.NumField(); i++ {
		vfield := v.Field(i)
		tfield := v.Type().Field(i)
		tag := tfield.Tag.Get(InfluxTag)
		tagfields := strings.Split(tag, ",")
		if len(tagfields) == 0 || len(tagfields) > 2 {
			stdlog.Println("tag field ignored: " + tag)
			continue
		}
		tagtype := tagfields[0]
		tagname := strings.ToLower(tfield.Name)
		if len(tagfields) == 2 {
			tagname = tagfields[1]
		}
		switch tagtype {
		case "key":
			keyParts[utils.MustAtoi(tagname)] = utils.ValToString(vfield.Interface(), "_", false)
		case "tag":
			ip.Tags[tagname] = utils.ValToString(vfield.Interface(), "\\,", noarray)
		case "value":
			ip.Fields[valuefield] = utils.ValToString(vfield.Interface(), ",", true) + "i"
		case "time":
			ip.Timestamp = vfield.Int()
		default:
		}
	}
	// sort key part keys and join them
	ip.Key = utils.Join(keyParts, "_")
	return &ip
}

// ConvertToKV converts a map[string]string to a csv with k=v pairs
func ConvertToKV(values map[string]string) string {
	var tmp []string
	for key, val := range values {
		if len(val) == 0 {
			continue
		}
		tmp = append(tmp, fmt.Sprintf("%s=%s", key, val))
	}
	return strings.Join(tmp, ",")
}

// ToInflux converts the influx point to influx string format
func (ip *InfluxPoint) ToInflux(noarray bool, valuefield string) string {
	return fmt.Sprintf("%s,%s %s %s", ip.Key, ConvertToKV(ip.Tags), ConvertToKV(ip.Fields), strconv.FormatInt(ip.Timestamp, 10))
}

// ToInflux serialises the data to be consumed by influx line protocol
// see https://docs.influxdata.com/influxdb/v1.2/write_protocols/line_protocol_tutorial/
func (p *Point) ToInflux(noarray bool, valuefield string) string {
	return p.GetInfluxPoint(noarray, valuefield).ToInflux(noarray, valuefield)
}

// CreateIndexIfNotExists not exists creates Elasticsearch Index if Not Exists
func CreateIndexIfNotExists(e *elastic.Client, index string) error {
	// Use the IndexExists service to check if a specified index exists.
	exists, err := e.IndexExists(index).Do(context.Background())
	if err != nil {
		errlog.Println("Unable to check if Elastic Index exists: ", err)
		return err
	}

	if !exists {
		// Create a new index.
		v := reflect.TypeOf(Point{})

		mapping := MapStr{
			"mappings": MapStr{
				"doc": MapStr{
					"properties": MapStr{},
				},
			},
		}

		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			tag := field.Tag.Get("elastic")
			if len(tag) == 0 {
				continue
			}
			tagfields := strings.Split(tag, ",")

			mapping["mappings"].(MapStr)["doc"].(MapStr)["properties"].(MapStr)[field.Name] = MapStr{}

			for _, tagfield := range tagfields {
				tagfieldValues := strings.Split(tagfield, ":")
				mapping["mappings"].(MapStr)["doc"].(MapStr)["properties"].(MapStr)[field.Name].(MapStr)[tagfieldValues[0]] = tagfieldValues[1]
			}
		}
		mappingJSON, err := json.Marshal(mapping)

		if err != nil {
			errlog.Println("Error on Json Marshal")
			return err
		}

		_, err = e.CreateIndex(index).BodyString(string(mappingJSON)).Do(context.Background())

		if err != nil {
			errlog.Println("Error creating Elastic Index:" + index)
			return err
		}
		stdlog.Println("Elastic Index created: " + index)
	}
	return nil
}

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

// InitPrometheus : Set some channels to notify other theads when using Prometheus
func (backend *Config) InitPrometheus(channel *chan bool, doneChannel *chan bool, promMetrics *chan Point) error {
	backend.channel = channel
	backend.doneChannel = doneChannel
	backend.promMetrics = promMetrics
	return nil
}

// Describe : Implementation of Prometheus Collector.Describe
func (backend *Config) Describe(ch chan<- *prometheus.Desc) {
	prometheus.NewGauge(prometheus.GaugeOpts{Name: "Dummy", Help: "Dummy"}).Describe(ch)
}

// Collect : Implementation of Prometheus Collector.Collect
func (backend *Config) Collect(ch chan<- prometheus.Metric) {

	stdlog.Println("Requested Metrics!")

	*backend.channel <- true

	for {
		select {
		case point := <-*backend.promMetrics:
			labelNames := []string{"vcenter", "name"}
			var labelValues []string

			labelValues = append(labelValues, point.VCenter)
			labelValues = append(labelValues, point.ObjectName)
			if point.Group == "disk" || point.Group == "datastore" {
				if backend.NoArray {
					if len(point.Datastore) > 0 {
						labelNames = append(labelNames, "datastore")
						labelValues = append(labelValues, point.Datastore[0])
					}
				} else {
					if len(point.Datastore) > 0 {
						labelNames = append(labelNames, "datastore")
						labelValues = append(labelValues, strings.Join(point.Datastore, ","))
					}
				}
			}
			if point.Group == "net" {
				if backend.NoArray {
					if len(point.Network) > 0 {
						labelNames = append(labelNames, "network")
						labelValues = append(labelValues, point.Network[0])
					}
				} else {
					if len(point.Network) > 0 {
						labelNames = append(labelNames, "network")
						labelValues = append(labelValues, strings.Join(point.Network, ","))
					}
				}
			}
			if len(point.ESXi) > 0 {
				labelNames = append(labelNames, "host")
				labelValues = append(labelValues, point.ESXi)
			}
			if len(point.Cluster) > 0 {
				labelNames = append(labelNames, "cluster")
				labelValues = append(labelValues, point.Cluster)
			}
			if len(point.Instance) > 0 {
				labelNames = append(labelNames, "instance")
				labelValues = append(labelValues, point.Instance)
			}
			if len(point.ResourcePool) > 0 {
				labelNames = append(labelNames, "resourcepool")
				labelValues = append(labelValues, point.ResourcePool)
			}
			if len(point.Folder) > 0 {
				labelNames = append(labelNames, "folder")
				labelValues = append(labelValues, point.Folder)
			}
			if backend.NoArray {
				if len(point.ViTags) > 0 {
					labelNames = append(labelNames, "vitags")
					labelValues = append(labelValues, point.ViTags[0])
				}
			} else {
				if len(point.ViTags) > 0 {
					labelNames = append(labelNames, "vitags")
					labelValues = append(labelValues, strings.Join(point.ViTags, ","))
				}
			}
			//if point.NumCPU != 0 {
			//	labelNames = append(labelNames, "numcpu")
			//	labelValues = append(labelValues, strconv.FormatInt(int64(point.NumCPU), 10))
			//}
			//if point.MemorySizeMB != 0 {
			//	labelNames = append(labelNames, "memorysizemb")
			//	labelValues = append(labelValues, strconv.FormatInt(int64(point.MemorySizeMB), 10))
			//}
			name := "vsphere_" + point.ObjectType + "_" + point.Group + "_" + point.Counter + "_" + point.Rollup
			desc := prometheus.NewDesc(name, "vSphere collected metric", labelNames, nil)
			metric, err := prometheus.NewConstMetric(desc, prometheus.GaugeValue, float64(point.Value), labelValues...)
			if err != nil {
				errlog.Println("E! Error creating prometheus metric")
			}
			ch <- metric
		case <-*backend.doneChannel:
			return
		}
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
			if point == nil {
				continue
			}
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
