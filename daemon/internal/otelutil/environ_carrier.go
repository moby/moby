package otelutil

import (
	"os"
)

const (
	traceParentKey = "traceparent"
	traceStateKey  = "tracestate"

	// See https://github.com/open-telemetry/opentelemetry-specification/issues/740
	// and https://github.com/open-telemetry/oteps/pull/258.
	traceParentEnvVar = "TRACEPARENT"
	traceStateEnvVar  = "TRACESTATE"
)

type EnvironCarrier struct {
	TraceParent, TraceState string
}

// Get returns the value associated with the passed key.
func (c *EnvironCarrier) Get(key string) string {
	switch key {
	case traceParentKey:
		return c.TraceParent
	case traceStateKey:
		return c.TraceState
	}
	return ""
}

// Set stores the key-value pair.
func (c *EnvironCarrier) Set(key, value string) {
	switch key {
	case traceParentKey:
		c.TraceParent = value
	case traceStateKey:
		c.TraceState = value
	}
	// Other keys are not supported at this time.
}

// Keys lists the keys stored in this carrier.
func (c *EnvironCarrier) Keys() []string {
	var k []string
	if c.TraceParent != "" {
		k = append(k, traceParentKey)
	}
	if c.TraceState != "" {
		k = append(k, traceStateKey)
	}
	return k
}

func (c *EnvironCarrier) Environ() []string {
	var env []string
	if c.TraceParent != "" {
		env = append(env, traceParentEnvVar+"="+c.TraceParent)
	}
	if c.TraceState != "" {
		env = append(env, traceStateEnvVar+"="+c.TraceState)
	}
	return env
}

func PropagateFromEnvironment() *EnvironCarrier {
	return &EnvironCarrier{
		TraceParent: os.Getenv(traceParentEnvVar),
		TraceState:  os.Getenv(traceStateEnvVar),
	}
}
