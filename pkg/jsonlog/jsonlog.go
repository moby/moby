package jsonlog

import (
	"time"
)

// JSONLog represents a log message, typically a single entry from a given log stream.
// JSONLogs can be easily serialized to and from JSON and support custom formatting.
type JSONLog struct {
	// Log is the log message
	Log string `json:"log,omitempty"`
	// Stream is the log source
	Stream string `json:"stream,omitempty"`
	// Created is the created timestamp of log
	Created time.Time `json:"time"`
	// Attrs is the list of extra attributes provided by the user
	Attrs map[string]string `json:"attrs,omitempty"`
}

// Reset all fields to their zero value.
func (jl *JSONLog) Reset() {
	jl.Log = ""
	jl.Stream = ""
	jl.Created = time.Time{}
	jl.Attrs = make(map[string]string)
}
