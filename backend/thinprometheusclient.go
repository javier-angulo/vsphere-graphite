package backend

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

// ThinPrometheusClient tries to export metrics to prometheus as simply as possible
type ThinPrometheusClient struct {
	Hostname string
	Port     int
	address  string
}

const defaultPort = 9155

// NewThinPrometheusClient creates a new thin prometheus
func NewThinPrometheusClient(server string, port int) (ThinPrometheusClient, error) {
	//create the port
	if port == 0 {
		port = defaultPort
	} else if port < 1000 || port > 65535 {
		return ThinPrometheusClient{}, errors.New("Port is not in a user range")
	}
	address := ""
	if len(server) > 0 {
		address = server
	}
	address = fmt.Sprintf("%s:%d", address, port)
	return ThinPrometheusClient{Hostname: server, Port: port, address: address}, nil
}

// ListenAndServe will start the listen thead for metric requests
func (client *ThinPrometheusClient) ListenAndServe() error {
	log.Printf("Start listening for metric reauest at %s\n", client.address)
	return fasthttp.ListenAndServe(client.address, fasthttp.CompressHandler(requestHandler))
}

func requestHandler(ctx *fasthttp.RequestCtx) {
	if string(ctx.Path()) != "/metrics" {
		ctx.Error("Unsupported path", fasthttp.StatusNotFound)
		return
	}
	if done == nil || query == nil || metrics == nil {
		ctx.Error("Channels are not properly initialized.", fasthttp.StatusFailedDependency)
		return
	}
	// create a buffer to organise metrics per type
	buffer := map[string][]string{}
	// start the queries
	*query <- true
	// start a timeout
	timeout := time.After(10 * time.Second)
	// wait for the results
	wait := true
	for wait {
		select {
		case point := <-*metrics:
			// reset timer
			timeout = time.After(10 * time.Second)
			// add point to the buffer
			addToThinPrometheusBuffer(buffer, &point)
		case <-*done:
			// finish consuming metrics and break loop
			log.Println("Thin Prometheus was signaled the end of the collection")
			wait = false
		case <-timeout:
			log.Println("Thin Prometheus was signaled a timeout")
			wait = false
		}
	}
	ctx.SetContentType("text/plain; charset=utf8")
	for key, vals := range buffer {
		fmt.Fprintf(ctx, "#HELP %s %s\n", key, strings.Replace(key, "_", " ", -1))
		fmt.Fprintf(ctx, "#TYPE %s gauge\n", key)
		for _, val := range vals {
			fmt.Fprintf(ctx, "%s%s\n", key, val)
		}
	}
}

func addToThinPrometheusBuffer(buffer map[string][]string, point *Point) {
	metric := fmt.Sprintf("%s_%s_%s_%s", prefix, point.Group, point.Counter, point.Rollup)
	tags := point.GetTags(false, ",")
	var tmp []string
	for key, val := range tags {
		if len(val) == 0 {
			continue
		}
		tmp = append(tmp, fmt.Sprintf("%s=\"%s\"", key, val))
	}
	strtags := strings.Join(tmp, ",")
	if buffer[metric] == nil {
		buffer[metric] = []string{fmt.Sprintf("{%s}%d", strtags, point.Value)}
	} else {
		buffer[metric] = append(buffer[metric], fmt.Sprintf("{%s}%d", strtags, point.Value))
	}
}
