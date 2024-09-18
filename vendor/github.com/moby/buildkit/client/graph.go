package client

import (
	"time"

	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
)

type Vertex struct {
	Digest        digest.Digest     `json:"digest,omitempty"`
	Inputs        []digest.Digest   `json:"inputs,omitempty"`
	Name          string            `json:"name,omitempty"`
	Started       *time.Time        `json:"started,omitempty"`
	Completed     *time.Time        `json:"completed,omitempty"`
	Cached        bool              `json:"cached,omitempty"`
	Error         string            `json:"error,omitempty"`
	ProgressGroup *pb.ProgressGroup `json:"progressGroup,omitempty"`
}

type VertexStatus struct {
	ID        string        `json:"id"`
	Vertex    digest.Digest `json:"vertex,omitempty"`
	Name      string        `json:"name,omitempty"`
	Total     int64         `json:"total,omitempty"`
	Current   int64         `json:"current"`
	Timestamp time.Time     `json:"timestamp,omitempty"`
	Started   *time.Time    `json:"started,omitempty"`
	Completed *time.Time    `json:"completed,omitempty"`
}

type VertexLog struct {
	Vertex    digest.Digest `json:"vertex,omitempty"`
	Stream    int           `json:"stream,omitempty"`
	Data      []byte        `json:"data"`
	Timestamp time.Time     `json:"timestamp"`
}

type VertexWarning struct {
	Vertex digest.Digest `json:"vertex,omitempty"`
	Level  int           `json:"level,omitempty"`
	Short  []byte        `json:"short,omitempty"`
	Detail [][]byte      `json:"detail,omitempty"`
	URL    string        `json:"url,omitempty"`

	SourceInfo *pb.SourceInfo `json:"sourceInfo,omitempty"`
	Range      []*pb.Range    `json:"range,omitempty"`
}

type SolveStatus struct {
	Vertexes []*Vertex        `json:"vertexes,omitempty"`
	Statuses []*VertexStatus  `json:"statuses,omitempty"`
	Logs     []*VertexLog     `json:"logs,omitempty"`
	Warnings []*VertexWarning `json:"warnings,omitempty"`
}

type SolveResponse struct {
	// ExporterResponse is also used for CacheExporter
	ExporterResponse map[string]string
}
