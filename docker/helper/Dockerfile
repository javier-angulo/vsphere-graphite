FROM golang:alpine
LABEL maintainer="cblomart@gmail.com"

RUN apk add --no-cache make gcc git upx docker musl-dev

RUN go get -u golang.org/x/sys/windows/registry;\
	go get -u github.com/takama/daemon;\
	go get -u golang.org/x/net/context;\
	go get -u github.com/vmware/govmomi;\
	go get -u github.com/marpaia/graphite-golang;\
	go get -u github.com/influxdata/influxdb/client/v2;\
	go get -u github.com/pquerna/ffjson/fflib/v1;\
	go get -u code.cloudfoundry.org/bytefmt;\
	go get -u github.com/pquerna/ffjson;\
	go get -u github.com/olivere/elastic;\
	go get -u github.com/prometheus/client_golang/prometheus;\
	go get -u github.com/fluent/fluent-logger-golang/fluent;\
    go clean -cache