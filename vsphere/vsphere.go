package vsphere

import (
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cblomart/vsphere-graphite/backend"
	"github.com/cblomart/vsphere-graphite/utils"

	"golang.org/x/net/context"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

var cache Cache

// VCenter description
type VCenter struct {
	Hostname     string
	Username     string
	Password     string
	MetricGroups []*MetricGroup
}

// AddMetric : add a metric definition to a metric group
func (vcenter *VCenter) AddMetric(metric *MetricDef, mtype string) {
	// find the metric group for the type
	var metricGroup *MetricGroup
	for _, tmp := range vcenter.MetricGroups {
		if tmp.ObjectType == mtype {
			metricGroup = tmp
			break
		}
	}
	// create a new group if needed
	if metricGroup == nil {
		metricGroup = &MetricGroup{ObjectType: mtype}
		vcenter.MetricGroups = append(vcenter.MetricGroups, metricGroup)
	}
	// check if Metric already present
	for _, tmp := range metricGroup.Metrics {
		if tmp.Key == metric.Key {
			// metric already in metric group
			return
		}
	}
	metricGroup.Metrics = append(metricGroup.Metrics, metric)
}

// Connect : Conncet to vcenter
func (vcenter *VCenter) Connect() (*govmomi.Client, error) {

	// prepare vcname
	vcName := strings.Split(vcenter.Hostname, ".")[0]

	// Prepare vCenter Connections
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	log.Printf("vcenter %s: connecting\n", vcName)
	username := url.QueryEscape(vcenter.Username)
	password := url.QueryEscape(vcenter.Password)
	u, err := url.Parse("https://" + username + ":" + password + "@" + vcenter.Hostname + "/sdk")
	if err != nil {
		log.Printf("vcenter %s: could not parse vcenter url - %s\n", vcName, err)
		return nil, err
	}
	client, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		log.Printf("vcenter %s: could not connect to vcenter - %s\n", vcName, err)
		return nil, err
	}
	return client, nil
}

// InitMetrics : maps metric keys to requested metrics
func InitMetrics(metrics []*Metric, perfmanager *mo.PerformanceManager) {
	// build a map of perf key per metric string id
	metricToPerf := make(map[string]int32)
	for _, perf := range perfmanager.PerfCounter {
		key := fmt.Sprintf("%s.%s.%s", perf.GroupInfo.GetElementDescription().Key, perf.NameInfo.GetElementDescription().Key, string(perf.RollupType))
		metricToPerf[key] = perf.Key
	}
	// fill in the metric key
	for _, metric := range metrics {
		for _, metricdef := range metric.Definition {
			if key, ok := metricToPerf[metricdef.Metric]; ok {
				metricdef.Key = key
			}
		}
	}
}

// Init : initialize vcenter
func (vcenter *VCenter) Init(metrics []*Metric) {

	// prepare vcname
	vcName := strings.Split(vcenter.Hostname, ".")[0]

	log.Printf("vcenter %s: initializing\n", vcName)
	// connect to vcenter
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, err := vcenter.Connect()
	if err != nil {
		log.Printf("vcenter %s: could not connect to vcenter - %s\n", vcName, err)
		return
	}
	defer func() {
		log.Printf("vcenter %s: disconnecting\n", vcName)
		err := client.Logout(ctx) // nolint: vetshadow
		if err != nil {
			log.Printf("vcenter %s: error logging out - %s\n", vcName, err)
		}
	}()
	// get the performance manager
	var perfmanager mo.PerformanceManager
	err = client.RetrieveOne(ctx, *client.ServiceContent.PerfManager, nil, &perfmanager)
	if err != nil {
		log.Printf("vcenter %s: could not get performance manager - %s\n", vcName, err)
		return
	}
	InitMetrics(metrics, &perfmanager)
	// add the metric to be collected in vcenter
	for _, metric := range metrics {
		for _, metricdef := range metric.Definition {
			if metricdef.Key == 0 {
				log.Printf("vcenter %s: metric key was not found %s", vcName, metricdef.Metric)
				continue
			}
			for _, mtype := range metric.ObjectType {
				vcenter.AddMetric(metricdef, mtype)
			}
		}
	}
	// initialize cache
	if cache == nil {
		cache = make(Cache)
	}
}

// Query : Query a vcenter
func (vcenter *VCenter) Query(interval int, domain string, replacepoint bool, properties []string, channel *chan backend.Point, wg *sync.WaitGroup) {
	defer func() {
		if wg != nil {
			wg.Done()
		}
	}()

	// prepare vcname
	vcName := strings.Replace(vcenter.Hostname, domain, "", -1)

	log.Printf("vcenter %s: setting up query inventory", vcName)

	// Create the contect
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get the client
	client, err := vcenter.Connect()
	if err != nil {
		log.Printf("vcenter %s: could not connect to vcenter - %s\n", vcName, err)
		return
	}

	// wait to be properly connected to defer logout
	defer func() {
		log.Printf("vcenter %s: disconnecting\n", vcName)
		err := client.Logout(ctx) // nolint: vetshadow
		if err != nil {
			log.Printf("vcenter %s: error logging out - %s\n", vcName, err)
		}
	}()

	// Create the view manager
	var viewManager mo.ViewManager
	err = client.RetrieveOne(ctx, *client.ServiceContent.ViewManager, nil, &viewManager)
	if err != nil {
		log.Printf("vcenter %s: could not get view manager - %s\n", vcName, err)
		return
	}

	// Find all Datacenters accessed by the user
	datacenters := []types.ManagedObjectReference{}
	finder := find.NewFinder(client.Client, true)
	dcs, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		log.Printf("vcenter %s: could not find a datacenter - %s\n", vcName, err)
		return
	}
	for _, child := range dcs {
		datacenters = append(datacenters, child.Reference())
	}

	// get the object types from properties
	objectTypes := []string{}
	for _, property := range properties {
		if propval, ok := Properties[property]; ok {
			for objkey := range propval {
				objectTypes = append(objectTypes, objkey)
			}
		}
	}
	// Get interesting object types from specified queries
	// Complete object of interest with required metrics
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
		res, err := methods.CreateContainerView(ctx, client.RoundTripper, &req) // nolint: vetshadow
		if err != nil {
			log.Printf("vcenter %s: could not create container view - %s\n", vcName, err)
			continue
		}
		// Retrieve the created ContentView
		var containerView mo.ContainerView
		err = client.RetrieveOne(ctx, res.Returnval, nil, &containerView)
		if err != nil {
			log.Printf("vcenter %s: could not get container - %s\n", vcName, err)
			continue
		}
		// Add found object to object list
		mors = append(mors, containerView.View...)
	}

	if len(mors) == 0 {
		log.Printf("vcenter %s: no object of intereset in types %s\n", vcName, strings.Join(objectTypes, ", "))
		return
	}

	//object for propery collection
	var objectSet []types.ObjectSpec
	for _, mor := range mors {
		objectSet = append(objectSet, types.ObjectSpec{Obj: mor, Skip: types.NewBool(false)})
	}

	//properties specifications
	reqProps := make(map[string][]string)
	//first fill in name for each object type
	for _, objType := range objectTypes {
		reqProps[objType] = []string{"name"}
	}
	//complete with required properties
	for _, property := range properties {
		if typeProperties, ok := Properties[property]; ok {
			for objType, typeProperties := range typeProperties {
				for _, typeProperty := range typeProperties {
					found := false
					for _, prop := range reqProps[objType] {
						if prop == typeProperty {
							found = true
							break
						}
					}
					if !found {
						reqProperties := append(reqProps[objType], typeProperty)
						reqProps[objType] = reqProperties
					}
				}
			}
		}
	}
	propSet := []types.PropertySpec{}
	for objType, props := range reqProps {
		propSet = append(propSet, types.PropertySpec{Type: objType, PathSet: props})
	}

	//retrieve properties
	propreq := types.RetrieveProperties{SpecSet: []types.PropertyFilterSpec{{ObjectSet: objectSet, PropSet: propSet}}}
	propres, err := client.PropertyCollector().RetrieveProperties(ctx, propreq)
	if err != nil {
		log.Printf("vcenter %s: could not retrieve object names - %s\n", vcName, err)
		return
	}

	// fill in the sections from queried data
	refs := make([]string, len(propres.Returnval))
	for _, objectContent := range propres.Returnval {
		refs = append(refs, objectContent.Obj.Value)
		for _, Property := range objectContent.PropSet {
			if section, ok := PropertiesSections[Property.Name]; ok {
				cache.Add(vcName, section, objectContent.Obj.Value, Property.Val)
			} else {
				log.Printf("vcenter %s: unhandled property '%s' for %s whose type is '%T'\n", vcName, Property.Name, objectContent.Obj.Value, Property.Val)
			}
		}
	}

	// cleanup buffer
	cache.CleanAll(vcName, refs)

	// create a map to resolve metric names
	metricKeys := []string{}
	for _, metricgroup := range vcenter.MetricGroups {
		for _, metricdef := range metricgroup.Metrics {
			i := utils.ValToString(metricdef.Key, "", true)
			cache.Add(vcName, "metrics", i, metricdef.Metric)
			metricKeys = append(metricKeys, i)
		}
	}
	// empty metric to names
	cache.Clean(vcName, "metrics", metricKeys)

	//create a map to resolve vm to their ressourcepool
	poolvms := []string{}
	for mor, vmmors := range *cache.LookupMorefs(vcName, "vms") {
		// only parse resource pools
		if strings.HasPrefix(mor, "rp-") {
			// find the full path of the resource pool
			poolmor := mor
			pools := []string{}
			for {
				poolname := cache.GetString(vcName, "names", poolmor)
				if len(*poolname) == 0 {
					// could not find name
					log.Printf("vcenter %s: could not find name for resourcepool %s\n", vcName, poolmor)
					break
				}
				if *poolname == "Resources" {
					// ignore root resourcepool
					break
				}
				// add the name to the path
				pools = append(pools, *poolname)
				newmor := cache.GetString(vcName, "parents", poolmor)
				if len(*newmor) == 0 {
					// no parent pool found
					log.Printf("vcenter %s: could not find parent for resourcepool %s\n", vcName, *poolname)
					break
				}
				poolmor = *newmor
				if !strings.HasPrefix(poolmor, "rp-") {
					break
				}
			}
			utils.Reverse(pools)
			poolpath := strings.Join(pools, "/")
			for _, vmmor := range *vmmors {
				if vmmor.Type == "VirtualMachine" {
					poolvms = append(poolvms, vmmor.Value)
					// Check if value already correct
					curpath := cache.GetString(vcName, "poolpaths", vmmor.Value)
					if curpath != nil {
						if poolpath == *curpath {
							continue
						}
					}
					// assign it
					cache.Add(vcName, "poolpaths", vmmor.Value, poolpath)
				}
			}
		}
	}
	// cleanup poolpaths
	cache.Clean(vcName, "poolpaths", poolvms)

	// create a map to resolve datastore ids to their names
	datastoreids := []string{}
	for mor, dsurl := range *cache.LookupString(vcName, "urls") {
		// find the datastore id
		regex := regexp.MustCompile("/([A-Fa-f0-9-]+)/$")
		matches := regex.FindAllString(*dsurl, 1)
		datastoreid := ""
		if len(matches) == 1 {
			datastoreid = strings.Trim(matches[0], "/")
		}
		// if an id is found, search for the name
		name := cache.GetString(vcName, "names", mor)
		if name != nil {
			// add the value to the index of ids
			datastoreids = append(datastoreids, datastoreid)
			// add the value to the cache
			cache.Add(vcName, "datastoreids", datastoreid, *name)
		}
	}
	cache.Clean(vcName, "datastoreids", datastoreids)

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

	// Check that there is something to query
	querycount := len(queries)
	if querycount == 0 {
		log.Printf("vcenter %s: no queries created!", vcName)
		return
	}
	metriccount := 0
	for _, query := range queries {
		metriccount = metriccount + len(query.MetricId)
	}
	log.Printf("vcenter %s: issuing %d queries requesting %d metrics.\n", vcName, querycount, metriccount)

	// Query the performances
	perfreq := types.QueryPerf{This: *client.ServiceContent.PerfManager, QuerySpec: queries}
	perfres, err := methods.QueryPerf(ctx, client.RoundTripper, &perfreq)
	if err != nil {
		log.Printf("vcenter %s: could not request perfs - %s\n", vcName, err)
		return
	}

	// Get the result
	returncount := len(perfres.Returnval)
	if returncount == 0 {
		log.Printf("vcenter %s: no result returned by queries\n", vcName)
		return
	}
	log.Printf("vcenter %s: retuned %d metrics\n", vcName, returncount)

	// create an array to store vm to folder path resolution
	valuescount := 0
	cache.Purge(vcName, "folders")

	for _, base := range perfres.Returnval {
		pem := base.(*types.PerfEntityMetric)
		// name checks the name of the object
		name := cache.FindString(vcName, "names", pem.Entity.Value)
		name = strings.ToLower(strings.Replace(name, domain, "", -1))
		if replacepoint {
			name = strings.Replace(name, ".", "_", -1)
		}
		//find datastore
		datastore := cache.FindNames(vcName, "datastores", pem.Entity.Value)
		//find host and cluster
		vmhost, cluster := cache.FindHostAndCluster(vcName, pem.Entity.Value)
		vmhost = strings.Replace(vmhost, domain, "", -1)
		//find network
		network := cache.FindNames(vcName, "networks", pem.Entity.Value)
		//find resource pool path
		resourcepool := cache.FindName(vcName, "poolpaths", pem.Entity.Value)
		//find folder path
		paths := []string{}
		if strings.HasPrefix(pem.Entity.Value, "vm-") {
			current := cache.GetString(vcName, "parents", pem.Entity.Value)
			for {
				if current == nil {
					break
				}
				if !(strings.HasPrefix(*current, "folder-") || strings.HasPrefix(*current, "group-")) {
					break
				}
				foldername := cache.GetString(vcName, "names", *current)
				if foldername == nil {
					log.Printf("vcenter %s: folder name not found for %s\n", vcName, *current)
					break
				}
				if *foldername == "vm" {
					break
				}
				paths = append(paths, *foldername)
				current = cache.GetString(vcName, "parents", *current)
			}
		}
		utils.Reverse(paths)
		folderpath := strings.Join(paths, "/")
		//find tags
		vitags := cache.FindTags(vcName, pem.Entity.Value)
		if len(pem.Value) == 0 {
			log.Printf("vcenter %s: no values returned in metrics!", vcName)
		}
		objType := strings.ToLower(pem.Entity.Type)
		timeStamp := endTime.Unix()
		// replace point in vcname
		rvcname := vcName
		if replacepoint {
			rvcname = strings.Replace(rvcname, ".", "_", -1)
		}
		// prepare basic informaitons of point
		point := backend.Point{
			VCenter:      rvcname,
			ObjectType:   objType,
			ObjectName:   name,
			Datastore:    datastore,
			ESXi:         vmhost,
			Cluster:      cluster,
			Network:      network,
			ResourcePool: resourcepool,
			Folder:       folderpath,
			ViTags:       vitags,
			Timestamp:    timeStamp,
		}
		//send disk infos
		diskInfos := cache.GetDiskInfos(vcName, "disks", pem.Entity.Value)
		if diskInfos != nil {
			for _, diskInfo := range *diskInfos {
				// skip if no capacity
				if diskInfo.Capacity == 0 {
					continue
				}
				// format disk path
				diskPath := strings.Replace(diskInfo.DiskPath, "\\", "/", -1)
				// send free space
				point.Group = "guestdisk"
				point.Counter = "freespace"
				point.Instance = diskPath
				point.Rollup = "latest"
				point.Value = diskInfo.FreeSpace
				*channel <- point
				// send capacity
				point.Group = "guestdisk"
				point.Counter = "capacity"
				point.Instance = diskPath
				point.Rollup = "latest"
				point.Value = diskInfo.Capacity
				*channel <- point
				// send usage %
				point.Group = "guestdisk"
				point.Counter = "usage"
				point.Instance = diskPath
				point.Rollup = "latest"
				point.Value = int64(10000 * (1 - (float64(diskInfo.FreeSpace) / float64(diskInfo.Capacity))))
				*channel <- point
			}
		}
		// send numcpu infos
		numcpu := cache.GetInt32(vcName, "cpus", pem.Entity.Value)
		if numcpu != nil {
			point.Group = "cpu"
			point.Counter = "count"
			point.Instance = ""
			point.Rollup = "latest"
			point.Value = int64(*numcpu)
			*channel <- point
		}
		// send numcpu infos
		memorysizemb := cache.GetInt32(vcName, "memories", pem.Entity.Value)
		if memorysizemb != nil {
			point.Group = "mem"
			point.Counter = "sizemb"
			point.Instance = ""
			point.Rollup = "latest"
			point.Value = int64(*memorysizemb)
			*channel <- point
		}
		valuescount = valuescount + len(pem.Value)
		for _, baseserie := range pem.Value {
			serie := baseserie.(*types.PerfMetricIntSeries)
			metricName := cache.FindMetricName(vcName, serie.Id.CounterId)
			metricName = strings.ToLower(metricName)
			metricparts := strings.Split(metricName, ".")
			point.Group, point.Counter, point.Rollup = metricparts[0], metricparts[1], metricparts[2]
			point.Instance = serie.Id.Instance
			if len(point.Instance) > 0 && point.Group == "datastore" {
				newDatastore := cache.GetString(vcName, "datastoreids", point.Instance)
				if newDatastore != nil {
					point.Datastore = []string{*newDatastore}
				} else {
					point.Datastore = []string{}
				}
			}
			var value int64 = -1
			switch {
			case strings.HasSuffix(metricName, ".average"):
				value = utils.Average(serie.Value...)
			case strings.HasSuffix(metricName, ".maximum"):
				value = utils.Max(serie.Value...)
			case strings.HasSuffix(metricName, ".minimum"):
				value = utils.Min(serie.Value...)
			case strings.HasSuffix(metricName, ".latest"):
				value = serie.Value[len(serie.Value)-1]
			case strings.HasSuffix(metricName, ".summation"):
				value = utils.Sum(serie.Value...)
			}
			point.Value = value
			*channel <- point
		}
	}
	log.Printf("vcenter %s: got %d results with %d values", vcName, returncount, valuescount)
}
