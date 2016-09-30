package executor

import (
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	clustertypes "github.com/docker/docker/daemon/cluster/provider"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/cluster"
	networktypes "github.com/docker/libnetwork/types"
	"golang.org/x/net/context"
)

// Backend defines the executor component for a swarm agent.
type Backend interface {
	CreateManagedNetwork(clustertypes.NetworkCreateRequest) error
	DeleteManagedNetwork(name string) error
	FindNetwork(idName string) (libnetwork.Network, error)
	SetupIngress(req clustertypes.NetworkCreateRequest, nodeIP string) error
	PullImage(ctx context.Context, image, tag string, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error
	CreateManagedContainer(config types.ContainerCreateConfig, validateHostname bool) (types.ContainerCreateResponse, error)
	ContainerStart(name string, hostConfig *container.HostConfig, validateHostname bool, checkpoint string) error
	ContainerStop(name string, seconds int) error
	ConnectContainerToNetwork(containerName, networkName string, endpointConfig *network.EndpointSettings) error
	UpdateContainerServiceConfig(containerName string, serviceConfig *clustertypes.ServiceConfig) error
	ContainerInspectCurrent(name string, size bool) (*types.ContainerJSON, error)
	ContainerWaitWithContext(ctx context.Context, name string) error
	ContainerRm(name string, config *types.ContainerRmConfig) error
	ContainerKill(name string, sig uint64) error
	SystemInfo() (*types.Info, error)
	VolumeCreate(name, driverName string, opts, labels map[string]string) (*types.Volume, error)
	Containers(config *types.ContainerListOptions) ([]*types.Container, error)
	SetNetworkBootstrapKeys([]*networktypes.EncryptionKey) error
	SetClusterProvider(provider cluster.Provider)
	IsSwarmCompatible() error
	SubscribeToEvents(since, until time.Time, filter filters.Args) ([]events.Message, chan interface{})
	UnsubscribeFromEvents(listener chan interface{})
	UpdateAttachment(string, string, string, *network.NetworkingConfig) error
	WaitForDetachment(context.Context, string, string, string, string) error
}
