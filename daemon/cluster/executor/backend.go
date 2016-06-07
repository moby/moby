package executor

import (
	"io"

	clustertypes "github.com/docker/docker/daemon/cluster/provider"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/engine-api/types/network"
	networktypes "github.com/docker/libnetwork/types"
	"golang.org/x/net/context"
)

// Backend defines the executor component for a swarm agent.
type Backend interface {
	CreateAgentNetwork(clustertypes.NetworkCreateRequest) error
	DeleteAgentNetwork(name string) error
	SetupIngress(req clustertypes.NetworkCreateRequest, nodeIP string) error
	PullImage(ctx context.Context, image, tag string, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error
	ContainerCreate(types.ContainerCreateConfig) (types.ContainerCreateResponse, error)
	ContainerStart(name string, hostConfig *container.HostConfig) error
	ContainerStop(name string, seconds int) error
	ConnectContainerToNetwork(containerName, networkName string, endpointConfig *network.EndpointSettings) error
	UpdateContainerServiceConfig(containerName string, serviceConfig *clustertypes.ServiceConfig) error
	ContainerInspectCurrent(name string, size bool) (*types.ContainerJSON, error)
	ContainerWaitWithContext(ctx context.Context, name string) (<-chan int, error)
	ContainerRm(name string, config *types.ContainerRmConfig) error
	ContainerKill(name string, sig uint64) error
	SystemInfo() (*types.Info, error)
	VolumeCreate(name, driverName string, opts, labels map[string]string) (*types.Volume, error)
	ListContainersForNode(nodeID string) []string
	SetNetworkBootstrapKeys([]*networktypes.EncryptionKey) error
}
