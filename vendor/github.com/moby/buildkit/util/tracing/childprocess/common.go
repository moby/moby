package childprocess

const (
	// go.opentelemetry.io/otel/propagation doesn't export these as constants.
	traceparentHeader = "traceparent"
	tracestateHeader  = "tracestate"
)

type textMap struct {
	parent string
	state  string
}

func (tm *textMap) Get(key string) string {
	switch key {
	case traceparentHeader:
		return tm.parent
	case tracestateHeader:
		return tm.state
	default:
		return ""
	}
}

func (tm *textMap) Set(key string, value string) {
	switch key {
	case traceparentHeader:
		tm.parent = value
	case tracestateHeader:
		tm.state = value
	}
}

func (tm *textMap) Keys() []string {
	var k []string
	if tm.parent != "" {
		k = append(k, traceparentHeader)
	}
	if tm.state != "" {
		k = append(k, tracestateHeader)
	}
	return k
}
