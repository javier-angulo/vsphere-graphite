package backend

import (
	"log"
	"reflect"
	"strings"

	"github.com/cblomart/vsphere-graphite/utils"
)

// Point : Information collected for a point
type Point struct {
	VCenter      string   `influx:"tag,vcenter"`
	ObjectType   string   `influx:"tag,type"`
	ObjectName   string   `influx:"tag,name"`
	Group        string   `influx:"key,1"`
	Counter      string   `influx:"key,2"`
	Instance     string   `influx:"tag,instance"`
	Rollup       string   `influx:"key,3"`
	Value        int64    `influx:"value"`
	Datastore    []string `influx:"tag,datastore" json:",omitempty"`
	ESXi         string   `influx:"tag,host" json:",omitempty"`
	Cluster      string   `influx:"tag,cluster" json:",omitempty"`
	Network      []string `influx:"tag,network" json:",omitempty"`
	ResourcePool string   `influx:"tag,resourcepool" json:",omitempty"`
	Folder       string   `influx:"tag,folder" json:",omitempty"`
	ViTags       []string `influx:"tag,vitags" json:",omitempty"`
	Timestamp    int64    `influx:"time" elastic:"type:date,format:epoch_second"`
}

// GetTags returns the tags of point as a map of strings
func (point *Point) GetTags(noarray bool, separator string) map[string]string {
	tags := map[string]string{}
	tags["vcenter"] = point.VCenter
	tags["type"] = point.ObjectType
	tags["name"] = point.ObjectName
	if noarray {
		if len(point.Datastore) > 0 {
			tags["datastore"] = point.Datastore[0]
		}
	} else {
		if len(point.Datastore) > 0 {
			tags["datastore"] = strings.Join(point.Datastore, separator)
		}
	}
	if noarray {
		if len(point.Network) > 0 {
			tags["network"] = point.Network[0]
		}
	} else {
		if len(point.Network) > 0 {
			tags["network"] = strings.Join(point.Network, separator)
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
	if noarray {
		if len(point.ViTags) > 0 {
			tags["vitags"] = point.ViTags[0]
		}
	} else {
		if len(point.ViTags) > 0 {
			tags["vitags"] = strings.Join(point.ViTags, separator)
		}
	}
	/*
		if point.NumCPU != 0 {
			tags["numcpu"] = strconv.FormatInt(int64(point.NumCPU), 10)
		}
		if point.MemorySizeMB != 0 {
			tags["memorysizemb"] = strconv.FormatInt(int64(point.MemorySizeMB), 10)
		}
	*/
	return tags
}

// GetInfluxPoint : convert a point to an influxpoint
func (point *Point) GetInfluxPoint(noarray bool, valuefield string) *InfluxPoint {
	keyParts := make(map[int]string)
	ip := InfluxPoint{
		Fields: make(map[string]string),
		Tags:   make(map[string]string),
	}
	v := reflect.ValueOf(point).Elem()
	for i := 0; i < v.NumField(); i++ {
		vfield := v.Field(i)
		tfield := v.Type().Field(i)
		tag := tfield.Tag.Get(InfluxTag)
		tagfields := strings.Split(tag, ",")
		if len(tagfields) == 0 || len(tagfields) > 2 {
			log.Println("tag field ignored: " + tag)
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
			value := utils.ValToString(vfield.Interface(), ",", noarray)
			// need to escape " " and "," in tags
			value = strings.Replace(value, ",", "\\,", -1)
			value = strings.Replace(value, " ", "\\ ", -1)
			ip.Tags[tagname] = value
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

// ToInflux serialises the data to be consumed by influx line protocol
// see https://docs.influxdata.com/influxdb/v1.2/write_protocols/line_protocol_tutorial/
func (point *Point) ToInflux(noarray bool, valuefield string) string {
	return point.GetInfluxPoint(noarray, valuefield).ToInflux(noarray, valuefield)
}
