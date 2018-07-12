package vsphere

import "github.com/vmware/govmomi/vim25/types"

// MetricGroup Grouping for retrieval
type MetricGroup struct {
	ObjectType string
	Metrics    []*MetricDef
	Mor        []*types.ManagedObjectReference
}

// MetricDef Definition
type MetricDef struct {
	Metric    string
	Instances string
	Key       int32
}

// Metric description in config
type Metric struct {
	ObjectType []string
	Definition []*MetricDef
}
