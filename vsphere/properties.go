package vsphere

// Properties describes know relation to properties to related objects and properties
var Properties = map[string]map[string][]string{
	"datastore": map[string][]string{
		"Datastore":      []string{"name"},
		"VirtualMachine": []string{"datastore"},
	},
	"host": map[string][]string{
		"HostSystem":     []string{"name", "parent"},
		"VirtualMachine": []string{"name", "runtime.host"},
	},
	"cluster": map[string][]string{
		"ClusterComputeResource": []string{"name"},
	},
	"network": map[string][]string{
		"DistributedVirtualPortgroup": []string{"name"},
		"Network":                     []string{"name"},
		"VirtualMachine":              []string{"network"},
	},
	"resourcepool": map[string][]string{
		"ResourcePool": []string{"name", "parent", "vm"},
	},
	"folder": map[string][]string{
		"Folder":         []string{"name", "parent"},
		"VirtualMachine": []string{"parent"},
	},
	"tags": map[string][]string{
		"VirtualMachine": []string{"tag"},
		"HostSystem":     []string{"tag"},
	},
	"numcpu": map[string][]string{
		"VirtualMachine": []string{"summary.config.numCpu"},
	},
	"memorysizemb": map[string][]string{
		"VirtualMachine": []string{"summary.config.memorySizeMB"},
	},
	"disks": map[string][]string{
		"VirtualMachine": []string{"guest.disk"},
	},
}
