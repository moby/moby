package executor

import (
	"context"
	"io"
	"time"

	"github.com/distribution/reference"
	"github.com/docker/distribution"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/api/types/volume"
	clustertypes "github.com/moby/moby/v2/daemon/cluster/provider"
	containerpkg "github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/internal/image"
	"github.com/moby/moby/v2/daemon/libnetwork"
	"github.com/moby/moby/v2/daemon/libnetwork/cluster"
	networktypes "github.com/moby/moby/v2/daemon/libnetwork/types"
	networkSettings "github.com/moby/moby/v2/daemon/network"
	"github.com/moby/moby/v2/daemon/pkg/plugin"
	"github.com/moby/moby/v2/daemon/server/backend"
	volumeopts "github.com/moby/moby/v2/daemon/volume/service/opts"
	"github.com/moby/swarmkit/v2/agent/exec"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Backend defines the executor component for a swarm agent.
type Backend interface {
	CreateManagedNetwork(clustertypes.NetworkCreateRequest) error
	DeleteManagedNetwork(networkID string) error
	FindNetwork(idName string) (*libnetwork.Network, error)
	SetupIngress(clustertypes.NetworkCreateRequest, string) (<-chan struct{}, error)
	ReleaseIngress() (<-chan struct{}, error)
	CreateManagedContainer(ctx context.Context, config backend.ContainerCreateConfig) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, name string, checkpoint string, checkpointDir string) error
	ContainerStop(ctx context.Context, name string, config container.StopOptions) error
	ContainerLogs(ctx context.Context, name string, config *container.LogsOptions) (msgs <-chan *backend.LogMessage, tty bool, err error)
	ConnectContainerToNetwork(ctx context.Context, containerName, networkName string, endpointConfig *network.EndpointSettings) error
	ActivateContainerServiceBinding(containerName string) error
	DeactivateContainerServiceBinding(containerName string) error
	UpdateContainerServiceConfig(containerName string, serviceConfig *clustertypes.ServiceConfig) error
	ContainerInspect(ctx context.Context, name string, options backend.ContainerInspectOptions) (*container.InspectResponse, error)
	ContainerWait(ctx context.Context, name string, condition container.WaitCondition) (<-chan containerpkg.StateStatus, error)
	ContainerRm(name string, config *backend.ContainerRmConfig) error
	ContainerKill(name string, sig string) error
	SetContainerDependencyStore(name string, store exec.DependencyGetter) error
	SetContainerSecretReferences(name string, refs []*swarm.SecretReference) error
	SetContainerConfigReferences(name string, refs []*swarm.ConfigReference) error
	SystemInfo(context.Context) (*system.Info, error)
	Containers(ctx context.Context, config *container.ListOptions) ([]*container.Summary, error)
	SetNetworkBootstrapKeys([]*networktypes.EncryptionKey) error
	DaemonJoinsCluster(provider cluster.Provider)
	DaemonLeavesCluster()
	IsSwarmCompatible() error
	SubscribeToEvents(since, until time.Time, filter filters.Args) ([]events.Message, chan any)
	UnsubscribeFromEvents(listener chan any)
	UpdateAttachment(string, string, string, *network.NetworkingConfig) error
	WaitForDetachment(context.Context, string, string, string, string) error
	PluginManager() *plugin.Manager
	PluginGetter() *plugin.Store
	GetAttachmentStore() *networkSettings.AttachmentStore
	HasExperimental() bool
}

// VolumeBackend is used by an executor to perform volume operations
type VolumeBackend interface {
	Create(ctx context.Context, name, driverName string, opts ...volumeopts.CreateOption) (*volume.Volume, error)
}

// ImageBackend is used by an executor to perform image operations
type ImageBackend interface {
	PullImage(ctx context.Context, ref reference.Named, platform *ocispec.Platform, metaHeaders map[string][]string, authConfig *registry.AuthConfig, outStream io.Writer) error
	GetRepositories(context.Context, reference.Named, *registry.AuthConfig) ([]distribution.Repository, error)
	GetImage(ctx context.Context, refOrID string, options backend.GetImageOpts) (*image.Image, error)
}
