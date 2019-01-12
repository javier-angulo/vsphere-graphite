package backend

import (
	"github.com/cblomart/vsphere-graphite/backend/thininfluxclient"
	"github.com/fluent/fluent-logger-golang/fluent"

	// removed until influxdb client comes back
	//influxclient "github.com/influxdata/influxdb/client/v2"
	graphite "github.com/marpaia/graphite-golang"
	"github.com/olivere/elastic"
)

// Config : storage backend
type Config struct {
	Hostname   string
	ValueField string
	Database   string
	Username   string
	Password   string
	Type       string
	Prefix     string
	Port       int
	NoArray    bool
	Encrypted  bool
	carbon     *graphite.Graphite
	// removed until influxdb client comes back
	//influx       *influxclient.Client
	thininfluxdb *thininfluxclient.ThinInfluxClient
	elastic      *elastic.Client
	fluent       *fluent.Fluent
}
