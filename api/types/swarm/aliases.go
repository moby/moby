package swarm

import "github.com/moby/moby/api/types/swarm"

// Version represents the internal object version.
type Version = swarm.Version

// Meta is a base object inherited by most of the other once.
type Meta = swarm.Meta

// Annotations represents how to describe an object.
type Annotations = swarm.Annotations

// Driver represents a driver (network, logging, secrets backend).
type Driver = swarm.Driver

// TLSInfo represents the TLS information about what CA certificate is trusted,
// and who the issuer for a TLS certificate is
type TLSInfo = swarm.TLSInfo

// Config represents a config.
type Config = swarm.Config

// ConfigSpec represents a config specification from a config in swarm
type ConfigSpec = swarm.ConfigSpec

// ConfigReferenceFileTarget is a file target in a config reference
type ConfigReferenceFileTarget = swarm.ConfigReferenceFileTarget

// ConfigReferenceRuntimeTarget is a target for a config specifying that it
// isn't mounted into the container but instead has some other purpose.
type ConfigReferenceRuntimeTarget = swarm.ConfigReferenceRuntimeTarget

// ConfigReference is a reference to a config in swarm
type ConfigReference = swarm.ConfigReference

// ConfigCreateResponse contains the information returned to a client
// on the creation of a new config.
type ConfigCreateResponse = swarm.ConfigCreateResponse

// ConfigListOptions holds parameters to list configs
type ConfigListOptions = swarm.ConfigListOptions

// DNSConfig specifies DNS related configurations in resolver configuration file (resolv.conf)
type DNSConfig = swarm.DNSConfig

// SELinuxContext contains the SELinux labels of the container.
type SELinuxContext = swarm.SELinuxContext

// SeccompMode is the type used for the enumeration of possible seccomp modes
// in SeccompOpts
type SeccompMode = swarm.SeccompMode

const (
	SeccompModeDefault    = swarm.SeccompModeDefault
	SeccompModeUnconfined = swarm.SeccompModeUnconfined
	SeccompModeCustom     = swarm.SeccompModeCustom
)

// SeccompOpts defines the options for configuring seccomp on a swarm-managed
// container.
type SeccompOpts = swarm.SeccompOpts

// AppArmorMode is type used for the enumeration of possible AppArmor modes in
// AppArmorOpts
type AppArmorMode = swarm.AppArmorMode

const (
	AppArmorModeDefault  = swarm.AppArmorModeDefault
	AppArmorModeDisabled = swarm.AppArmorModeDisabled
)

// AppArmorOpts defines the options for configuring AppArmor on a swarm-managed
// container.  Currently, custom AppArmor profiles are not supported.
type AppArmorOpts = swarm.AppArmorOpts

// CredentialSpec for managed service account (Windows only)
type CredentialSpec = swarm.CredentialSpec

// Privileges defines the security options for the container.
type Privileges = swarm.Privileges

// ContainerSpec represents the spec of a container.
type ContainerSpec = swarm.ContainerSpec

// Endpoint represents an endpoint.
type Endpoint = swarm.Endpoint

// EndpointSpec represents the spec of an endpoint.
type EndpointSpec = swarm.EndpointSpec

// ResolutionMode represents a resolution mode.
type ResolutionMode = swarm.ResolutionMode

const (
	// ResolutionModeVIP VIP
	ResolutionModeVIP = swarm.ResolutionModeVIP
	// ResolutionModeDNSRR DNSRR
	ResolutionModeDNSRR = swarm.ResolutionModeDNSRR
)

// PortConfig represents the config of a port.
type PortConfig = swarm.PortConfig

// PortConfigPublishMode represents the mode in which the port is to
// be published.
type PortConfigPublishMode = swarm.PortConfigPublishMode

const (
	// PortConfigPublishModeIngress is used for ports published
	// for ingress load balancing using routing mesh.
	PortConfigPublishModeIngress = swarm.PortConfigPublishModeIngress
	// PortConfigPublishModeHost is used for ports published
	// for direct host level access on the host where the task is running.
	PortConfigPublishModeHost = swarm.PortConfigPublishModeHost
)

// PortConfigProtocol represents the protocol of a port.
type PortConfigProtocol = swarm.PortConfigProtocol

const (
	// PortConfigProtocolTCP TCP
	PortConfigProtocolTCP = swarm.PortConfigProtocolTCP
	// PortConfigProtocolUDP UDP
	PortConfigProtocolUDP = swarm.PortConfigProtocolUDP
	// PortConfigProtocolSCTP SCTP
	PortConfigProtocolSCTP = swarm.PortConfigProtocolSCTP
)

// EndpointVirtualIP represents the virtual ip of a port.
type EndpointVirtualIP = swarm.EndpointVirtualIP

// Network represents a network.
type Network = swarm.Network

// NetworkSpec represents the spec of a network.
type NetworkSpec = swarm.NetworkSpec

// NetworkAttachmentConfig represents the configuration of a network attachment.
type NetworkAttachmentConfig = swarm.NetworkAttachmentConfig

// NetworkAttachment represents a network attachment.
type NetworkAttachment = swarm.NetworkAttachment

// IPAMOptions represents ipam options.
type IPAMOptions = swarm.IPAMOptions

// IPAMConfig represents ipam configuration.
type IPAMConfig = swarm.IPAMConfig

// Node represents a node.
type Node = swarm.Node

// NodeSpec represents the spec of a node.
type NodeSpec = swarm.NodeSpec

// NodeRole represents the role of a node.
type NodeRole = swarm.NodeRole

const (
	// NodeRoleWorker WORKER
	NodeRoleWorker = swarm.NodeRoleWorker
	// NodeRoleManager MANAGER
	NodeRoleManager = swarm.NodeRoleManager
)

// NodeAvailability represents the availability of a node.
type NodeAvailability = swarm.NodeAvailability

const (
	// NodeAvailabilityActive ACTIVE
	NodeAvailabilityActive = swarm.NodeAvailabilityActive
	// NodeAvailabilityPause PAUSE
	NodeAvailabilityPause = swarm.NodeAvailabilityPause
	// NodeAvailabilityDrain DRAIN
	NodeAvailabilityDrain = swarm.NodeAvailabilityDrain
)

// NodeDescription represents the description of a node.
type NodeDescription = swarm.NodeDescription

// Platform represents the platform (Arch/OS).
type Platform = swarm.Platform

// EngineDescription represents the description of an engine.
type EngineDescription = swarm.EngineDescription

// NodeCSIInfo represents information about a CSI plugin available on the node
type NodeCSIInfo = swarm.NodeCSIInfo

// PluginDescription represents the description of an engine plugin.
type PluginDescription = swarm.PluginDescription

// NodeStatus represents the status of a node.
type NodeStatus = swarm.NodeStatus

// Reachability represents the reachability of a node.
type Reachability = swarm.Reachability

const (
	// ReachabilityUnknown UNKNOWN
	ReachabilityUnknown = swarm.ReachabilityUnknown
	// ReachabilityUnreachable UNREACHABLE
	ReachabilityUnreachable = swarm.ReachabilityUnreachable
	// ReachabilityReachable REACHABLE
	ReachabilityReachable = swarm.ReachabilityReachable
)

// ManagerStatus represents the status of a manager.
type ManagerStatus = swarm.ManagerStatus

// NodeState represents the state of a node.
type NodeState = swarm.NodeState

const (
	// NodeStateUnknown UNKNOWN
	NodeStateUnknown = swarm.NodeStateUnknown
	// NodeStateDown DOWN
	NodeStateDown = swarm.NodeStateDown
	// NodeStateReady READY
	NodeStateReady = swarm.NodeStateReady
	// NodeStateDisconnected DISCONNECTED
	NodeStateDisconnected = swarm.NodeStateDisconnected
)

// Topology defines the CSI topology of this node. This type is a duplicate of
// github.com/docker/docker/api/types.Topology. Because the type definition
// is so simple and to avoid complicated structure or circular imports, we just
// duplicate it here. See that type for full documentation
type Topology = swarm.Topology

// NodeListOptions holds parameters to list nodes with.
type NodeListOptions = swarm.NodeListOptions

// NodeRemoveOptions holds parameters to remove nodes with.
type NodeRemoveOptions = swarm.NodeRemoveOptions

// RuntimeType is the type of runtime used for the TaskSpec
type RuntimeType = swarm.RuntimeType

// RuntimeURL is the proto type url
type RuntimeURL = swarm.RuntimeURL

const (
	// RuntimeContainer is the container based runtime
	RuntimeContainer = swarm.RuntimeContainer
	// RuntimePlugin is the plugin based runtime
	RuntimePlugin = swarm.RuntimePlugin
	// RuntimeNetworkAttachment is the network attachment runtime
	RuntimeNetworkAttachment = swarm.RuntimeNetworkAttachment

	// RuntimeURLContainer is the proto url for the container type
	RuntimeURLContainer = swarm.RuntimeURLContainer
	// RuntimeURLPlugin is the proto url for the plugin type
	RuntimeURLPlugin = swarm.RuntimeURLPlugin
)

// NetworkAttachmentSpec represents the runtime spec type for network
// attachment tasks
type NetworkAttachmentSpec = swarm.NetworkAttachmentSpec

// Secret represents a secret.
type Secret = swarm.Secret

// SecretSpec represents a secret specification from a secret in swarm
type SecretSpec = swarm.SecretSpec

// SecretReferenceFileTarget is a file target in a secret reference
type SecretReferenceFileTarget = swarm.SecretReferenceFileTarget

// SecretReference is a reference to a secret in swarm
type SecretReference = swarm.SecretReference

// SecretCreateResponse contains the information returned to a client
// on the creation of a new secret.
type SecretCreateResponse = swarm.SecretCreateResponse

// SecretListOptions holds parameters to list secrets
type SecretListOptions = swarm.SecretListOptions

// Service represents a service.
type Service = swarm.Service

// ServiceSpec represents the spec of a service.
type ServiceSpec = swarm.ServiceSpec

// ServiceMode represents the mode of a service.
type ServiceMode = swarm.ServiceMode

// UpdateState is the state of a service update.
type UpdateState = swarm.UpdateState

const (
	// UpdateStateUpdating is the updating state.
	UpdateStateUpdating = swarm.UpdateStateUpdating
	// UpdateStatePaused is the paused state.
	UpdateStatePaused = swarm.UpdateStatePaused
	// UpdateStateCompleted is the completed state.
	UpdateStateCompleted = swarm.UpdateStateCompleted
	// UpdateStateRollbackStarted is the state with a rollback in progress.
	UpdateStateRollbackStarted = swarm.UpdateStateRollbackStarted
	// UpdateStateRollbackPaused is the state with a rollback in progress.
	UpdateStateRollbackPaused = swarm.UpdateStateRollbackPaused
	// UpdateStateRollbackCompleted is the state with a rollback in progress.
	UpdateStateRollbackCompleted = swarm.UpdateStateRollbackCompleted
)

// UpdateStatus reports the status of a service update.
type UpdateStatus = swarm.UpdateStatus

// ReplicatedService is a kind of ServiceMode.
type ReplicatedService = swarm.ReplicatedService

// GlobalService is a kind of ServiceMode.
type GlobalService = swarm.GlobalService

// ReplicatedJob is the a type of Service which executes a defined Tasks
// in parallel until the specified number of Tasks have succeeded.
type ReplicatedJob = swarm.ReplicatedJob

// GlobalJob is the type of a Service which executes a Task on every Node
// matching the Service's placement constraints. These tasks run to completion
// and then exit.
//
// This type is deliberately empty.
type GlobalJob = swarm.GlobalJob

const (
	// UpdateFailureActionPause PAUSE
	UpdateFailureActionPause = swarm.UpdateFailureActionPause
	// UpdateFailureActionContinue CONTINUE
	UpdateFailureActionContinue = swarm.UpdateFailureActionContinue
	// UpdateFailureActionRollback ROLLBACK
	UpdateFailureActionRollback = swarm.UpdateFailureActionRollback

	// UpdateOrderStopFirst STOP_FIRST
	UpdateOrderStopFirst = swarm.UpdateOrderStopFirst
	// UpdateOrderStartFirst START_FIRST
	UpdateOrderStartFirst = swarm.UpdateOrderStartFirst
)

// UpdateConfig represents the update configuration.
type UpdateConfig = swarm.UpdateConfig

// ServiceStatus represents the number of running tasks in a service and the
// number of tasks desired to be running.
type ServiceStatus = swarm.ServiceStatus

// JobStatus is the status of a job-type service.
type JobStatus = swarm.JobStatus

// ServiceCreateOptions contains the options to use when creating a service.
type ServiceCreateOptions = swarm.ServiceCreateOptions

// Values for RegistryAuthFrom in ServiceUpdateOptions
const (
	RegistryAuthFromSpec         = swarm.RegistryAuthFromSpec
	RegistryAuthFromPreviousSpec = swarm.RegistryAuthFromPreviousSpec
)

// ServiceUpdateOptions contains the options to be used for updating services.
type ServiceUpdateOptions = swarm.ServiceUpdateOptions

// ServiceListOptions holds parameters to list services with.
type ServiceListOptions = swarm.ServiceListOptions

// ServiceInspectOptions holds parameters related to the "service inspect"
// operation.
type ServiceInspectOptions = swarm.ServiceInspectOptions

// ServiceCreateResponse contains the information returned to a client on the
// creation of a new service.
type ServiceCreateResponse = swarm.ServiceCreateResponse

// ServiceUpdateResponse service update response
type ServiceUpdateResponse = swarm.ServiceUpdateResponse

// ClusterInfo represents info about the cluster for outputting in "info"
// it contains the same information as "Swarm", but without the JoinTokens
type ClusterInfo = swarm.ClusterInfo

// Swarm represents a swarm.
type Swarm = swarm.Swarm

// JoinTokens contains the tokens workers and managers need to join the swarm.
type JoinTokens = swarm.JoinTokens

// Spec represents the spec of a swarm.
type Spec = swarm.Spec

// OrchestrationConfig represents orchestration configuration.
type OrchestrationConfig = swarm.OrchestrationConfig

// TaskDefaults parameterizes cluster-level task creation with default values.
type TaskDefaults = swarm.TaskDefaults

// EncryptionConfig controls at-rest encryption of data and keys.
type EncryptionConfig = swarm.EncryptionConfig

// RaftConfig represents raft configuration.
type RaftConfig = swarm.RaftConfig

// DispatcherConfig represents dispatcher configuration.
type DispatcherConfig = swarm.DispatcherConfig

// CAConfig represents CA configuration.
type CAConfig = swarm.CAConfig

// ExternalCAProtocol represents type of external CA.
type ExternalCAProtocol = swarm.ExternalCAProtocol

// ExternalCAProtocolCFSSL CFSSL
const ExternalCAProtocolCFSSL = swarm.ExternalCAProtocolCFSSL

// ExternalCA defines external CA to be used by the cluster.
type ExternalCA = swarm.ExternalCA

// InitRequest is the request used to init a swarm.
type InitRequest = swarm.InitRequest

// JoinRequest is the request used to join a swarm.
type JoinRequest = swarm.JoinRequest

// UnlockRequest is the request used to unlock a swarm.
type UnlockRequest = swarm.UnlockRequest

// LocalNodeState represents the state of the local node.
type LocalNodeState = swarm.LocalNodeState

const (
	// LocalNodeStateInactive INACTIVE
	LocalNodeStateInactive = swarm.LocalNodeStateInactive
	// LocalNodeStatePending PENDING
	LocalNodeStatePending = swarm.LocalNodeStatePending
	// LocalNodeStateActive ACTIVE
	LocalNodeStateActive = swarm.LocalNodeStateActive
	// LocalNodeStateError ERROR
	LocalNodeStateError = swarm.LocalNodeStateError
	// LocalNodeStateLocked LOCKED
	LocalNodeStateLocked = swarm.LocalNodeStateLocked
)

// Info represents generic information about swarm.
type Info = swarm.Info

// Status provides information about the current swarm status and role,
// obtained from the "Swarm" header in the API response.
type Status = swarm.Status

// Peer represents a peer.
type Peer = swarm.Peer

// UpdateFlags contains flags for SwarmUpdate.
type UpdateFlags = swarm.UpdateFlags

// UnlockKeyResponse contains the response for Engine API:
// GET /swarm/unlockkey
type UnlockKeyResponse = swarm.UnlockKeyResponse

// TaskState represents the state of a task.
type TaskState = swarm.TaskState

const (
	// TaskStateNew NEW
	TaskStateNew = swarm.TaskStateNew
	// TaskStateAllocated ALLOCATED
	TaskStateAllocated = swarm.TaskStateAllocated
	// TaskStatePending PENDING
	TaskStatePending = swarm.TaskStatePending
	// TaskStateAssigned ASSIGNED
	TaskStateAssigned = swarm.TaskStateAssigned
	// TaskStateAccepted ACCEPTED
	TaskStateAccepted = swarm.TaskStateAccepted
	// TaskStatePreparing PREPARING
	TaskStatePreparing = swarm.TaskStatePreparing
	// TaskStateReady READY
	TaskStateReady = swarm.TaskStateReady
	// TaskStateStarting STARTING
	TaskStateStarting = swarm.TaskStateStarting
	// TaskStateRunning RUNNING
	TaskStateRunning = swarm.TaskStateRunning
	// TaskStateComplete COMPLETE
	TaskStateComplete = swarm.TaskStateComplete
	// TaskStateShutdown SHUTDOWN
	TaskStateShutdown = swarm.TaskStateShutdown
	// TaskStateFailed FAILED
	TaskStateFailed = swarm.TaskStateFailed
	// TaskStateRejected REJECTED
	TaskStateRejected = swarm.TaskStateRejected
	// TaskStateRemove REMOVE
	TaskStateRemove = swarm.TaskStateRemove
	// TaskStateOrphaned ORPHANED
	TaskStateOrphaned = swarm.TaskStateOrphaned
)

// Task represents a task.
type Task = swarm.Task

// TaskSpec represents the spec of a task.
type TaskSpec = swarm.TaskSpec

// Resources represents resources (CPU/Memory) which can be advertised by a
// node and requested to be reserved for a task.
type Resources = swarm.Resources

// Limit describes limits on resources which can be requested by a task.
type Limit = swarm.Limit

// GenericResource represents a "user defined" resource which can
// be either an integer (e.g: SSD=3) or a string (e.g: SSD=sda1)
type GenericResource = swarm.GenericResource

// NamedGenericResource represents a "user defined" resource which is defined
// as a string.
// "Kind" is used to describe the Kind of a resource (e.g: "GPU", "FPGA", "SSD", ...)
// Value is used to identify the resource (GPU="UUID-1", FPGA="/dev/sdb5", ...)
type NamedGenericResource = swarm.NamedGenericResource

// DiscreteGenericResource represents a "user defined" resource which is defined
// as an integer
// "Kind" is used to describe the Kind of a resource (e.g: "GPU", "FPGA", "SSD", ...)
// Value is used to count the resource (SSD=5, HDD=3, ...)
type DiscreteGenericResource = swarm.DiscreteGenericResource

// ResourceRequirements represents resources requirements.
type ResourceRequirements = swarm.ResourceRequirements

// Placement represents orchestration parameters.
type Placement = swarm.Placement

// PlacementPreference provides a way to make the scheduler aware of factors
// such as topology.
type PlacementPreference = swarm.PlacementPreference

// SpreadOver is a scheduling preference that instructs the scheduler to spread
// tasks evenly over groups of nodes identified by labels.
type SpreadOver = swarm.SpreadOver

// RestartPolicy represents the restart policy.
type RestartPolicy = swarm.RestartPolicy

// RestartPolicyCondition represents when to restart.
type RestartPolicyCondition = swarm.RestartPolicyCondition

const (
	// RestartPolicyConditionNone NONE
	RestartPolicyConditionNone = swarm.RestartPolicyConditionNone
	// RestartPolicyConditionOnFailure ON_FAILURE
	RestartPolicyConditionOnFailure = swarm.RestartPolicyConditionOnFailure
	// RestartPolicyConditionAny ANY
	RestartPolicyConditionAny = swarm.RestartPolicyConditionAny
)

// TaskStatus represents the status of a task.
type TaskStatus = swarm.TaskStatus

// ContainerStatus represents the status of a container.
type ContainerStatus = swarm.ContainerStatus

// PortStatus represents the port status of a task's host ports whose
// service has published host ports
type PortStatus = swarm.PortStatus

// VolumeAttachment contains the associating a Volume to a Task.
type VolumeAttachment = swarm.VolumeAttachment

// TaskListOptions holds parameters to list tasks with.
type TaskListOptions = swarm.TaskListOptions

// ---------------------------------------------------
// New types, introduced in moby/moby/api
// ---------------------------------------------------

type RuntimeSpec = swarm.RuntimeSpec

type RuntimePrivilege = swarm.RuntimePrivilege
