package client

import (
	"context"
	"io"
	"syscall"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/apicaps"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	fstypes "github.com/tonistiigi/fsutil/types"
)

type Client interface {
	Solve(ctx context.Context, req SolveRequest) (*Result, error)
	ResolveImageConfig(ctx context.Context, ref string, opt llb.ResolveImageConfigOpt) (digest.Digest, []byte, error)
	BuildOpts() BuildOpts
	Inputs(ctx context.Context) (map[string]llb.State, error)
	NewContainer(ctx context.Context, req NewContainerRequest) (Container, error)
	Warn(ctx context.Context, dgst digest.Digest, msg string, opts WarnOpts) error
}

// NewContainerRequest encapsulates the requirements for a client to define a
// new container, without defining the initial process.
type NewContainerRequest struct {
	Mounts      []Mount
	NetMode     pb.NetMode
	ExtraHosts  []*pb.HostIP
	Platform    *pb.Platform
	Constraints *pb.WorkerConstraints
}

// Mount allows clients to specify a filesystem mount. A Reference to a
// previously solved Result is required.
type Mount struct {
	Selector  string
	Dest      string
	ResultID  string
	Ref       Reference
	Readonly  bool
	MountType pb.MountType
	CacheOpt  *pb.CacheOpt
	SecretOpt *pb.SecretOpt
	SSHOpt    *pb.SSHOpt
}

// Container is used to start new processes inside a container and release the
// container resources when done.
type Container interface {
	Start(context.Context, StartRequest) (ContainerProcess, error)
	Release(context.Context) error
}

// StartRequest encapsulates the arguments to define a process within a
// container.
type StartRequest struct {
	Args           []string
	Env            []string
	User           string
	Cwd            string
	Tty            bool
	Stdin          io.ReadCloser
	Stdout, Stderr io.WriteCloser
	SecurityMode   pb.SecurityMode
}

// WinSize is same as executor.WinSize, copied here to prevent circular package
// dependencies.
type WinSize struct {
	Rows uint32
	Cols uint32
}

// ContainerProcess represents a process within a container.
type ContainerProcess interface {
	Wait() error
	Resize(ctx context.Context, size WinSize) error
	Signal(ctx context.Context, sig syscall.Signal) error
}

type Reference interface {
	ToState() (llb.State, error)
	ReadFile(ctx context.Context, req ReadRequest) ([]byte, error)
	StatFile(ctx context.Context, req StatRequest) (*fstypes.Stat, error)
	ReadDir(ctx context.Context, req ReadDirRequest) ([]*fstypes.Stat, error)
}

type ReadRequest struct {
	Filename string
	Range    *FileRange
}

type FileRange struct {
	Offset int
	Length int
}

type ReadDirRequest struct {
	Path           string
	IncludePattern string
}

type StatRequest struct {
	Path string
}

// SolveRequest is same as frontend.SolveRequest but avoiding dependency
type SolveRequest struct {
	Evaluate       bool
	Definition     *pb.Definition
	Frontend       string
	FrontendOpt    map[string]string
	FrontendInputs map[string]*pb.Definition
	CacheImports   []CacheOptionsEntry
}

type CacheOptionsEntry struct {
	Type  string
	Attrs map[string]string
}

type WorkerInfo struct {
	ID        string
	Labels    map[string]string
	Platforms []ocispecs.Platform
}

type BuildOpts struct {
	Opts      map[string]string
	SessionID string
	Workers   []WorkerInfo
	Product   string
	LLBCaps   apicaps.CapSet
	Caps      apicaps.CapSet
}

type WarnOpts struct {
	Level      int
	SourceInfo *pb.SourceInfo
	Range      []*pb.Range
	Detail     [][]byte
	URL        string
}
