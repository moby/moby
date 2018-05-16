package container

import "io"

// Stats contains response of Engine API:
// GET "/stats"
type Stats struct {
	Body   io.ReadCloser `json:"body"`
	OSType string        `json:"ostype"`
}
