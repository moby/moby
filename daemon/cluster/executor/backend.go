package executor

import (
	"io"
	"time"

	clustertypes "github.com/docker/docker/daemon/cluster/provider"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/engine-api/types/events"
	"github.com/docker/engine-api/types/filters"
	"github.com/docker/engine-api/types/network"
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
	CreateManagedContainer(config types.ContainerCreateConfig) (types.ContainerCreateResponse, error)
	ContainerStart(name string, hostConfig *container.HostConfig) error
	ContainerStop(name string, seconds int) error
	ConnectContainerToNetwork(containerName, networkName string, endpointConfig *network.EndpointSettings) error
	UpdateContainerServiceConfig(containerName string, serviceConfig *clustertypes.ServiceConfig) error
	ContainerInspectCurrent(name string, size bool) (*types.ContainerJSON, error)
	ContainerWaitWithContext(ctx context.Context, name string) error
	ContainerRm(name string, config *types.ContainerRmConfig) error
	ContainerKill(name string, sig uint64) error
	SystemInfo() (*types.Info, error)
	VolumeCreate(name, driverName string, opts, labels map[string]string) (*types.Volume, error)
	ListContainersForNode(nodeID string) []string
	SetNetworkBootstrapKeys([]*networktypes.EncryptionKey) error
	SetClusterProvider(provider cluster.Provider)
	IsSwarmCompatible() error
	SubscribeToEvents(since, until time.Time, filter filters.Args) ([]events.Message, chan interface{})
	UnsubscribeFromEvents(listener chan interface{})
}
