package build

import (
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/registry"
)

// BuilderVersion sets the version of underlying builder to use
type BuilderVersion string

const (
	// BuilderV1 is the first generation builder in docker daemon
	BuilderV1 BuilderVersion = "1"
	// BuilderBuildKit is builder based on moby/buildkit project
	BuilderBuildKit BuilderVersion = "2"
)

// Result contains the image id of a successful build.
type Result struct {
	ID string
}

// ImageBuildOptions holds the information
// necessary to build images.
type ImageBuildOptions struct {
	Tags           []string
	SuppressOutput bool
	RemoteContext  string
	NoCache        bool
	Remove         bool
	ForceRemove    bool
	PullParent     bool
	Isolation      container.Isolation
	CPUSetCPUs     string
	CPUSetMems     string
	CPUShares      int64
	CPUQuota       int64
	CPUPeriod      int64
	Memory         int64
	MemorySwap     int64
	CgroupParent   string
	NetworkMode    string
	ShmSize        int64
	Dockerfile     string
	Ulimits        []*container.Ulimit
	// BuildArgs needs to be a *string instead of just a string so that
	// we can tell the difference between "" (empty string) and no value
	// at all (nil). See the parsing of buildArgs in
	// api/server/router/build/build_routes.go for even more info.
	BuildArgs   map[string]*string
	AuthConfigs map[string]registry.AuthConfig
	Context     io.Reader
	Labels      map[string]string
	// squash the resulting image's layers to the parent
	// preserves the original image and creates a new one from the parent with all
	// the changes applied to a single layer
	Squash bool
	// CacheFrom specifies images that are used for matching cache. Images
	// specified here do not need to have a valid parent chain to match cache.
	CacheFrom   []string
	SecurityOpt []string
	ExtraHosts  []string // List of extra hosts
	Target      string
	SessionID   string
	Platform    string
	// Version specifies the version of the underlying builder to use
	Version BuilderVersion
	// BuildID is an optional identifier that can be passed together with the
	// build request. The same identifier can be used to gracefully cancel the
	// build with the cancel request.
	BuildID string
	// Outputs defines configurations for exporting build results. Only supported
	// in BuildKit mode
	Outputs []ImageBuildOutput
}

// ImageBuildOutput defines configuration for exporting a build result
type ImageBuildOutput struct {
	Type  string
	Attrs map[string]string
}

// ImageBuildResponse holds information
// returned by a server after building
// an image.
type ImageBuildResponse struct {
	Body   io.ReadCloser
	OSType string
}
