package client

import (
	"context"

	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

type Client interface {
	Solve(ctx context.Context, req SolveRequest) (*Result, error)
	ResolveImageConfig(ctx context.Context, ref string, opt ResolveImageConfigOpt) (digest.Digest, []byte, error)
	BuildOpts() BuildOpts
}

type Reference interface {
	ReadFile(ctx context.Context, req ReadRequest) ([]byte, error)
	// StatFile(ctx context.Context, req StatRequest) (*StatResponse, error)
	// ReadDir(ctx context.Context, req ReadDirRequest) ([]*StatResponse, error)
}

type ReadRequest struct {
	Filename string
	Range    *FileRange
}

type FileRange struct {
	Offset int
	Length int
}

// SolveRequest is same as frontend.SolveRequest but avoiding dependency
type SolveRequest struct {
	Definition      *pb.Definition
	Frontend        string
	FrontendOpt     map[string]string
	ImportCacheRefs []string
}

type WorkerInfo struct {
	ID        string
	Labels    map[string]string
	Platforms []specs.Platform
}

type BuildOpts struct {
	Opts      map[string]string
	SessionID string
	Workers   []WorkerInfo
	Product   string
}

type ResolveImageConfigOpt struct {
	Platform    *specs.Platform
	ResolveMode string
	LogName     string
}
