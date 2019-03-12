# vsphere-graphite

Altought it started out as a simple tool to pump vsphere statistics to graphite it was extended to support different backends:

* [prometheus](https://prometheus.io/)
* [influxdb](https://www.influxdata.com/)
* [fluentd](https://www.fluentd.org/)
* [elasticsearch](https://www.elastic.co/)
* [graphite](https://graphiteapp.org/)

It will basically get the requested statistics for each virtual machine or host and send them to the backend every minuts.

The configuration allows to specify per object types which statistics should be fetched.
By default cpu count and memory count and, when vmware tools are running, disk informations will be reported.

For the backends that supports it, extra metadata can be fetched:

* Datastores
* Networks
* Cluster
* ResourcePool
* Folderp

Sample reports:

![example elastic](https://github.com/cblomart/vsphere-graphite/raw/master/imgs/vsphere-graphite-elastic-grafana-dashboard-1.png)

![example influx](https://github.com/cblomart/vsphere-graphite/raw/master/imgs/vsphere-graphite-influxdb-grafana-dashboard-1.png)

![example prometheus](https://github.com/cblomart/vsphere-graphite/raw/master/imgs/vsphere-graphite-prometheus-grafana-dashboard-1.png)
