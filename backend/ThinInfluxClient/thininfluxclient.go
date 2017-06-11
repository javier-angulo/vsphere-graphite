package ThinInfluxClient

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

// constants
var precisions = []string{"ns", "u", "ms", "s", "m", "h"}
var maxlines = 5000

// InfluxError message returned on error
// ffjson: noencoder
type InfluxError struct {
	Error string
}

// ThinInfluxClient sends data to influxdb
// ffjson: skip
type ThinInfluxClient struct {
	URL      string
	Username string
	password string
}

// NewThinInlfuxClient creates a new thin influx client
func NewThinInlfuxClient(server string, port int, database, username, password, precision string, ssl bool) (ThinInfluxClient, error) {
	if len(server) == 0 {
		return ThinInfluxClient{}, errors.New("No url indicated")
	}
	if port < 1000 || port > 65535 {
		return ThinInfluxClient{}, errors.New("Port not in acceptable range")
	}
	if len(database) == 0 {
		return ThinInfluxClient{}, errors.New("No database indicated")
	}
	found := false
	for _, p := range precisions {
		if p == precision {
			found = true
			break
		}
	}
	if !found {
		return ThinInfluxClient{}, errors.New("Precision '" + precision + "' not in suppoted presisions " + strings.Join(precisions, ","))
	}
	fullurl := "http"
	if ssl {
		fullurl += "s"
	}
	fullurl += "://" + server + ":" + strconv.Itoa(port) + "/write?db=" + database + "&precision=" + precision

	return ThinInfluxClient{URL: fullurl, Username: username, password: password}, nil
}

// Send data to influx db
// Data is represented by lines of influxdb lineprotocol
// see https://docs.influxdata.com/influxdb/v1.2/write_protocols/line_protocol_tutorial/
// Limiting submits to maxlines (currently 5000) items as in
// https://docs.influxdata.com/influxdb/v1.2/guides/writing_data/
func (client *ThinInfluxClient) Send(lines []string) error {
	if len(lines) > maxlines {
		// split push per maxlines
		for i := 0; i <= len(lines); i += maxlines {
			end := i + maxlines
			if end > len(lines) {
				end = len(lines)
			}
			flush := lines[i:end]
			err := client.Send(flush)
			if err != nil {
				return err
			}
		}
		return nil
	}
	// prepare the content
	var buf bytes.Buffer
	g := gzip.NewWriter(&buf)
	for _, l := range lines {
		if _, err := g.Write([]byte(l + "\n")); err != nil {
			return err
		}
	}
	if err := g.Close(); err != nil {
		return err
	}
	// prepare the request
	req, err := http.NewRequest("POST", client.URL, &buf)
	if err != nil {
		return err
	}
	req.SetBasicAuth(client.Username, client.password)
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Content-Encoding", "gzip")
	clt := &http.Client{}
	resp, err := clt.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode == 204 {
		return nil
	}
	jsonerr := InfluxError{}
	if resp.StatusCode == 400 || resp.StatusCode == 404 || resp.StatusCode == 500 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		jsonerr.UnmarshalJSON(body)
	}
	if resp.StatusCode == 400 {
		return errors.New("Influxdb Unacceptable request: " + jsonerr.Error)
	}
	if resp.StatusCode == 401 {
		return errors.New("Unauthorized access: check credentials and db")
	}
	if resp.StatusCode == 404 {
		return errors.New("Database not found: " + jsonerr.Error)
	}
	if resp.StatusCode == 500 {
		return errors.New("Server Busy: " + jsonerr.Error)
	}
	return nil
}
