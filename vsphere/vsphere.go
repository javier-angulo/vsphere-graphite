package vsphere

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cblomart/vsphere-graphite/backend"
	"github.com/cblomart/vsphere-graphite/utils"

	"golang.org/x/net/context"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

var stdlog, errlog *log.Logger

// VCenter description
type VCenter struct {
	Hostname     string
	Username     string
	Password     string
	MetricGroups []*MetricGroup
}

// MetricDef Definition
type MetricDef struct {
	Metric    string
	Instances string
	Key       int32
}

// MetricGroup Grouping for retrieval
type MetricGroup struct {
	ObjectType string
	Metrics    []MetricDef
	Mor        []types.ManagedObjectReference
}

// Metric description in config
type Metric struct {
	ObjectType []string
	Definition []MetricDef
}

// Connect : Conncet to vcenter
func (vcenter *VCenter) Connect() (*govmomi.Client, error) {
	// Prepare vCenter Connections
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stdlog.Println("connecting to vcenter: " + vcenter.Hostname)
	username := url.QueryEscape(vcenter.Username)
	password := url.QueryEscape(vcenter.Password)
	u, err := url.Parse("https://" + username + ":" + password + "@" + vcenter.Hostname + "/sdk")
	if err != nil {
		errlog.Println("Could not parse vcenter url: ", vcenter.Hostname)
		errlog.Println("Error: ", err)
		return nil, err
	}
	client, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		errlog.Println("Could not connect to vcenter: ", vcenter.Hostname)
		errlog.Println("Error: ", err)
		return nil, err
	}
	return client, nil
}

// Init : initialize vcenter
func (vcenter *VCenter) Init(metrics []Metric, standardLogs *log.Logger, errorLogs *log.Logger) {
	stdlog = standardLogs
	errlog = errorLogs
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, err := vcenter.Connect()
	if err != nil {
		errlog.Println("Could not connect to vcenter: ", vcenter.Hostname)
		errlog.Println("Error: ", err)
		return
	}
	defer client.Logout(ctx)
	var perfmanager mo.PerformanceManager
	err = client.RetrieveOne(ctx, *client.ServiceContent.PerfManager, nil, &perfmanager)
	if err != nil {
		errlog.Println("Could not get performance manager")
		errlog.Println("Error: ", err)
		return
	}
	for _, perf := range perfmanager.PerfCounter {
		groupinfo := perf.GroupInfo.GetElementDescription()
		nameinfo := perf.NameInfo.GetElementDescription()
		identifier := groupinfo.Key + "." + nameinfo.Key + "." + fmt.Sprint(perf.RollupType)
		for _, metric := range metrics {
			for _, metricdef := range metric.Definition {
				if metricdef.Metric == identifier {
					metricd := MetricDef{Metric: metricdef.Metric, Instances: metricdef.Instances, Key: perf.Key}
					for _, mtype := range metric.ObjectType {
						added := false
						for _, metricgroup := range vcenter.MetricGroups {
							if metricgroup.ObjectType == mtype {
								metricgroup.Metrics = append(metricgroup.Metrics, metricd)
								stdlog.Println("Appended metric " + metricd.Metric + " identified by " + strconv.Itoa(int(metricd.Key)) + " to vcenter " + vcenter.Hostname + " for " + mtype)
								added = true
								break
							}
						}
						if !added {
							metricgroup := MetricGroup{ObjectType: mtype, Metrics: []MetricDef{metricd}}
							vcenter.MetricGroups = append(vcenter.MetricGroups, &metricgroup)
							stdlog.Println("Appended metric group with " + metricd.Metric + " identified by " + strconv.Itoa(int(metricd.Key)) + " to vcenter " + vcenter.Hostname + " for " + mtype)
						}
					}
				}
			}
		}
	}
}

// Query : Query a vcenter
func (vcenter *VCenter) Query(interval int, domain string, channel *chan []backend.Point) {
	stdlog.Println("Setting up query inventory of vcenter: ", vcenter.Hostname)

	// Create the contect
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get the client
	client, err := vcenter.Connect()
	if err != nil {
		errlog.Println("Could not connect to vcenter: ", vcenter.Hostname)
		errlog.Println("Error: ", err)
		return
	}

	// wait to be properly connected to defer logout
	defer client.Logout(ctx)

	// Create the view manager
	var viewManager mo.ViewManager
	err = client.RetrieveOne(ctx, *client.ServiceContent.ViewManager, nil, &viewManager)
	if err != nil {
		errlog.Println("Could not get view manager from vcenter: " + vcenter.Hostname)
		errlog.Println("Error: ", err)
		return
	}

	// Get the Datacenters from root folder
	var rootFolder mo.Folder
	err = client.RetrieveOne(ctx, client.ServiceContent.RootFolder, nil, &rootFolder)
	if err != nil {
		errlog.Println("Could not get root folder from vcenter: " + vcenter.Hostname)
		errlog.Println("Error: ", err)
		return
	}

	datacenters := []types.ManagedObjectReference{}
	for _, child := range rootFolder.ChildEntity {
		if child.Type == "Datacenter" {
			datacenters = append(datacenters, child)
		}
	}

	// Get intresting object types from specified queries
	objectTypes := []string{"ClusterComputeResource", "Datastore", "HostSystem", "DistributedVirtualPortgroup", "Network", "ResourcePool"}
	for _, group := range vcenter.MetricGroups {
		found := false
		for _, tmp := range objectTypes {
			if group.ObjectType == tmp {
				found = true
				break
			}
		}
		if !found {
			objectTypes = append(objectTypes, group.ObjectType)
		}
	}

	// Loop trought datacenters and create the intersting object reference list
	mors := []types.ManagedObjectReference{}
	for _, datacenter := range datacenters {
		// Create the CreateContentView request
		req := types.CreateContainerView{This: viewManager.Reference(), Container: datacenter, Type: objectTypes, Recursive: true}
		res, err := methods.CreateContainerView(ctx, client.RoundTripper, &req)
		if err != nil {
			errlog.Println("Could not create container view from vcenter: " + vcenter.Hostname)
			errlog.Println("Error: ", err)
			continue
		}
		// Retrieve the created ContentView
		var containerView mo.ContainerView
		err = client.RetrieveOne(ctx, res.Returnval, nil, &containerView)
		if err != nil {
			errlog.Println("Could not get container view from vcenter: " + vcenter.Hostname)
			errlog.Println("Error: ", err)
			continue
		}
		// Add found object to object list
		mors = append(mors, containerView.View...)
	}

	//object for propery collection
	var objectSet []types.ObjectSpec
	for _, mor := range mors {
		objectSet = append(objectSet, types.ObjectSpec{Obj: mor, Skip: types.NewBool(false)})
	}

	//properties specifications
	propSet := []types.PropertySpec{}
	propSet = append(propSet, types.PropertySpec{Type: "ManagedEntity", PathSet: []string{"name", "parent"}})
	propSet = append(propSet, types.PropertySpec{Type: "VirtualMachine", PathSet: []string{"datastore", "network", "runtime.host"}})
	propSet = append(propSet, types.PropertySpec{Type: "ResourcePool", PathSet: []string{"vm"}})

	//retrieve properties
	propreq := types.RetrieveProperties{SpecSet: []types.PropertyFilterSpec{{ObjectSet: objectSet, PropSet: propSet}}}
	propres, err := client.PropertyCollector().RetrieveProperties(ctx, propreq)
	if err != nil {
		errlog.Println("Could not retrieve object names from vcenter: " + vcenter.Hostname)
		errlog.Println("Error: ", err)
		return
	}

	//create a map to resolve object names
	morToName := make(map[types.ManagedObjectReference]string)

	//create a map to resolve vm to datastore
	vmToDatastore := make(map[types.ManagedObjectReference][]types.ManagedObjectReference)

	//create a map to resolve vm to network
	vmToNetwork := make(map[types.ManagedObjectReference][]types.ManagedObjectReference)

	//create a map to resolve vm to host
	vmToHost := make(map[types.ManagedObjectReference]types.ManagedObjectReference)

	//create a map to resolve host to parent - in a cluster the parent should be a cluster
	morToParent := make(map[types.ManagedObjectReference]types.ManagedObjectReference)

	//create a map to resolve mor to vms - object that contains multiple vms as child objects
	morToVms := make(map[types.ManagedObjectReference][]types.ManagedObjectReference)

	for _, objectContent := range propres.Returnval {
		for _, Property := range objectContent.PropSet {
			switch propertyName := Property.Name; propertyName {
			case "name":
				name, ok := Property.Val.(string)
				if ok {
					morToName[objectContent.Obj] = name
				} else {
					errlog.Println("Name property of " + objectContent.Obj.String() + " was not a string, it was " + fmt.Sprintf("%T", Property.Val))
				}
			case "datastore":
				mors, ok := Property.Val.(types.ArrayOfManagedObjectReference)
				if ok {
					if len(mors.ManagedObjectReference) > 0 {
						vmToDatastore[objectContent.Obj] = mors.ManagedObjectReference
					}
				} else {
					errlog.Println("Datastore property of " + objectContent.Obj.String() + " was not a ManagedObjectReferences, it was " + fmt.Sprintf("%T", Property.Val))
				}
			case "network":
				mors, ok := Property.Val.(types.ArrayOfManagedObjectReference)
				if ok {
					if len(mors.ManagedObjectReference) > 0 {
						vmToNetwork[objectContent.Obj] = mors.ManagedObjectReference
					}
				} else {
					errlog.Println("Network property of " + objectContent.Obj.String() + " was not an array of  ManagedObjectReferences, it was " + fmt.Sprintf("%T", Property.Val))
				}
			case "runtime.host":
				mor, ok := Property.Val.(types.ManagedObjectReference)
				if ok {
					vmToHost[objectContent.Obj] = mor
				} else {
					errlog.Println("Runtime host property of " + objectContent.Obj.String() + " was not a ManagedObjectReference, it was " + fmt.Sprintf("%T", Property.Val))
				}
			case "parent":
				mor, ok := Property.Val.(types.ManagedObjectReference)
				if ok {
					morToParent[objectContent.Obj] = mor
				} else {
					errlog.Println("Parent property of " + objectContent.Obj.String() + " was not a ManagedObjectReference, it was " + fmt.Sprintf("%T", Property.Val))
				}
			case "vm":
				mors, ok := Property.Val.(types.ArrayOfManagedObjectReference)
				if ok {
					if len(mors.ManagedObjectReference) > 0 {
						morToVms[objectContent.Obj] = mors.ManagedObjectReference
					}
				} else {
					errlog.Println("VM property of " + objectContent.Obj.String() + " was not an array of  ManagedObjectReferences, it was " + fmt.Sprintf("%T", Property.Val))
				}
			default:
				errlog.Println("Unhandled property '" + propertyName + "' for " + objectContent.Obj.String() + " whose type is " + fmt.Sprintf("%T", Property.Val))
			}
		}
	}

	//create a map to resolve metric names
	metricToName := make(map[int32]string)
	for _, metricgroup := range vcenter.MetricGroups {
		for _, metricdef := range metricgroup.Metrics {
			metricToName[metricdef.Key] = metricdef.Metric
		}
	}

	//create a map to resolve vm to their ressourcepool
	vmToResourcePoolPath := make(map[types.ManagedObjectReference]string)
	for mor, vmmors := range morToVms {
		// not doing case sensitive as this could be extensive
		stdlog.Println("Searching resourcepool path")
		stdlog.Println("resourcepool " + mor.String() + " with " + strconv.Itoa(len(vmmors)) + " vms")
		if mor.Type == "ResourcePool" {
			// find the full path of the resource pool
			poolpath := ""
			poolmor := mor
			ok := true
			for ok {
				poolname, ok := morToName[poolmor]
				if !ok {
					// could not find name
					errlog.Println("Could not find name for resourcepool " + mor.String())
					break
				}
				// add the name to the path
				poolpath = poolname + "/" + poolpath
				stdlog.Println("updated pool path: " + poolpath)
				prevmor := poolmor
				poolmor, ok := morToParent[poolmor]
				if !ok {
					// no parent pool found
					errlog.Println("Could not find parent for resourcepool " + poolname)
					break
				}
				if prevmor == poolmor {
					errlog.Println("Parent is oneself")
					break
				}
				if poolmor.Type != "ResourcePool" {
					break
				}
			}
			stdlog.Println("Found resourcepool path: " + poolpath)
			for _, vmmor := range vmmors {
				if vmmor.Type == "VirtualMachine" {
					vmToResourcePoolPath[vmmor] = poolpath
				}
			}
		}
	}

	// Create Queries from interesting objects and requested metrics

	queries := []types.PerfQuerySpec{}

	// Common parameters
	intervalID := int32(20)
	endTime := time.Now().Add(time.Duration(-1) * time.Second)
	startTime := endTime.Add(time.Duration(-interval-1) * time.Second)

	// Parse objects
	for _, mor := range mors {
		metricIds := []types.PerfMetricId{}
		for _, metricgroup := range vcenter.MetricGroups {
			if metricgroup.ObjectType == mor.Type {
				for _, metricdef := range metricgroup.Metrics {
					metricIds = append(metricIds, types.PerfMetricId{CounterId: metricdef.Key, Instance: metricdef.Instances})
				}
			}
		}
		if len(metricIds) > 0 {
			queries = append(queries, types.PerfQuerySpec{Entity: mor, StartTime: &startTime, EndTime: &endTime, MetricId: metricIds, IntervalId: intervalID})
		}
	}

	// Query the performances
	perfreq := types.QueryPerf{This: *client.ServiceContent.PerfManager, QuerySpec: queries}
	perfres, err := methods.QueryPerf(ctx, client.RoundTripper, &perfreq)
	if err != nil {
		errlog.Println("Could not request perfs from vcenter: " + vcenter.Hostname)
		errlog.Println("Error: ", err)
		return
	}

	// Get the result
	values := []backend.Point{}
	vcName := strings.Replace(vcenter.Hostname, domain, "", -1)
	for _, base := range perfres.Returnval {
		pem := base.(*types.PerfEntityMetric)
		//entityName := strings.ToLower(pem.Entity.Type)
		name := strings.ToLower(strings.Replace(morToName[pem.Entity], domain, "", -1))
		//find datastore
		datastore := []string{}
		if mors, ok := vmToDatastore[pem.Entity]; ok {
			for _, mor := range mors {
				datastore = append(datastore, morToName[mor])
			}
		}
		//find host and cluster
		vmhost := ""
		cluster := ""
		if pem.Entity.Type == "VirtualMachine" {
			if esximor, ok := vmToHost[pem.Entity]; ok {
				vmhost = strings.ToLower(strings.Replace(morToName[esximor], domain, "", -1))
				if parmor, ok := morToParent[esximor]; ok {
					if parmor.Type == "ClusterComputeResource" {
						cluster = morToName[parmor]
					} else {
						errlog.Println("Parent of host " + vmhost + " was " + parmor.Type + "(" + parmor.Value + ")")
					}
				}
			}
		} else if pem.Entity.Type == "HostSystem" {
			//find cluster if entity is a host
			if parmor, ok := morToParent[pem.Entity]; ok {
				if parmor.Type == "ClusterComputeResource" {
					cluster = morToName[parmor]
				} else {
					errlog.Println("Parent of host " + name + " was " + parmor.Type + "(" + parmor.Value + ")")
				}
			}
		}
		//find network
		network := []string{}
		if mors, ok := vmToNetwork[pem.Entity]; ok {
			for _, mor := range mors {
				network = append(network, morToName[mor])
			}
		}
		//find resource pool path
		resourcepool := ""
		if rppath, ok := vmToResourcePoolPath[pem.Entity]; ok {
			resourcepool = rppath
		}
		for _, baseserie := range pem.Value {
			serie := baseserie.(*types.PerfMetricIntSeries)
			metricName := strings.ToLower(metricToName[serie.Id.CounterId])
			instanceName := serie.Id.Instance
			var value int64 = -1
			if strings.HasSuffix(metricName, ".average") {
				value = utils.Average(serie.Value...)
			} else if strings.HasSuffix(metricName, ".maximum") {
				value = utils.Max(serie.Value...)
			} else if strings.HasSuffix(metricName, ".minimum") {
				value = utils.Min(serie.Value...)
			} else if strings.HasSuffix(metricName, ".latest") {
				value = serie.Value[len(serie.Value)-1]
			} else if strings.HasSuffix(metricName, ".summation") {
				value = utils.Sum(serie.Value...)
			}
			metricparts := strings.Split(metricName, ".")
			point := backend.Point{
				VCenter:      vcName,
				ObjectType:   strings.ToLower(pem.Entity.Type),
				ObjectName:   name,
				Group:        metricparts[0],
				Counter:      metricparts[1],
				Instance:     instanceName,
				Rollup:       metricparts[2],
				Value:        value,
				Datastore:    datastore,
				ESXi:         vmhost,
				Cluster:      cluster,
				Network:      network,
				ResourcePool: resourcepool,
				Timestamp:    endTime.Unix(),
			}
			values = append(values, point)
		}
	}
	*channel <- values
}
