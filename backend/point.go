package backend

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
	NumCPU       int32    `influx:"tag,numcpu" json:",omitempty"`
	MemorySizeMB int32    `influx:"tag,memorysizemb" json:",omitempty"`
	Timestamp    int64    `influx:"time" elastic:"type:date,format:epoch_second"`
}
