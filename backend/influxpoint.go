package backend

import (
	"fmt"
	"strconv"

	"github.com/cblomart/vsphere-graphite/utils"
)

// InfluxPoint is the representation of the parts of a point for influx
type InfluxPoint struct {
	Key       string
	Fields    map[string]string
	Tags      map[string]string
	Timestamp int64
}

// ToInflux converts the influx point to influx string format
func (ip *InfluxPoint) ToInflux(noarray bool, valuefield string) string {
	return fmt.Sprintf("%s,%s %s %s", ip.Key, utils.ConvertToKV(ip.Tags), utils.ConvertToKV(ip.Fields), strconv.FormatInt(ip.Timestamp, 10))
}
