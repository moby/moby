package client

import (
	"context"

	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// TODO: make this take same options as LLBBridge. Add Return()
type Client interface {
	Solve(ctx context.Context, req SolveRequest, exporterAttr map[string][]byte, final bool) (Reference, error)
	ResolveImageConfig(ctx context.Context, ref string, platform *specs.Platform) (digest.Digest, []byte, error)
	Opts() map[string]string
	SessionID() string
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
