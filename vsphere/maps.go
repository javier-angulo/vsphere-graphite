package vsphere

import "github.com/vmware/govmomi/vim25/types"

var (
	//create a map to resolve object names
	morToName map[string]*string

	//create a map to resolve vm to datastore
	vmToDatastore map[string]*[]types.ManagedObjectReference

	//create a map to resolve vm to network
	vmToNetwork map[string]*[]types.ManagedObjectReference

	//create a map to resolve vm to host
	vmToHost map[string]*string

	//create a map to resolve host to parent - in a cluster the parent should be a cluster
	morToParent map[string]*string

	//create a map to resolve mor to vms - object that contains multiple vms as child objects
	morToVms map[string]*[]types.ManagedObjectReference

	//create a map to resolve mor to tags
	morToTags map[string]*[]types.Tag

	//create a map to resolve mor to numcpu
	morToNumCPU map[string]*int32

	//create a map to resolve mor to memorysizemb
	morToMemorySizeMB map[string]*int32

	//create a map to resolve diskinfos
	morToDiskInfos map[string]*[]types.GuestDiskInfo

	//resolve metrics to name
	metricToName map[int32]*string

	//crate a ma to resolve folders
	morToFolderPath map[string]*[]string
)
