package backend

import (
	"github.com/cblomart/vsphere-graphite/backend/thininfluxclient"
	influxclient "github.com/influxdata/influxdb/client/v2"
	graphite "github.com/marpaia/graphite-golang"
	"github.com/olivere/elastic"
)

// Config : storage backend
type Config struct {
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
	thininfluxdb *thininfluxclient.ThinInfluxClient
	elastic      *elastic.Client
}
