package metrics

// Unit represents the type or precision of a metric that is appended to
// the metrics fully qualified name
type Unit string

const (
	Nanoseconds Unit = "nanoseconds"
	Seconds     Unit = "seconds"
	Bytes       Unit = "bytes"
	Total       Unit = "total"
)
