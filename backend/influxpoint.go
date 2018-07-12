package backend

// InfluxPoint is the representation of the parts of a point for influx
type InfluxPoint struct {
	Key       string
	Fields    map[string]string
	Tags      map[string]string
	Timestamp int64
}
