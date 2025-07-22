package events

import "github.com/moby/moby/api/types/events"

// Type is used for event-types.
type Type = events.Type

// List of known event types.
const (
	BuilderEventType   = events.BuilderEventType   // BuilderEventType is the event type that the builder generates.
	ConfigEventType    = events.ConfigEventType    // ConfigEventType is the event type that configs generate.
	ContainerEventType = events.ContainerEventType // ContainerEventType is the event type that containers generate.
	DaemonEventType    = events.DaemonEventType    // DaemonEventType is the event type that daemon generate.
	ImageEventType     = events.ImageEventType     // ImageEventType is the event type that images generate.
	NetworkEventType   = events.NetworkEventType   // NetworkEventType is the event type that networks generate.
	NodeEventType      = events.NodeEventType      // NodeEventType is the event type that nodes generate.
	PluginEventType    = events.PluginEventType    // PluginEventType is the event type that plugins generate.
	SecretEventType    = events.SecretEventType    // SecretEventType is the event type that secrets generate.
	ServiceEventType   = events.ServiceEventType   // ServiceEventType is the event type that services generate.
	VolumeEventType    = events.VolumeEventType    // VolumeEventType is the event type that volumes generate.
)

// Action is used for event-actions.
type Action = events.Action

const (
	ActionCreate       = events.ActionCreate
	ActionStart        = events.ActionStart
	ActionRestart      = events.ActionRestart
	ActionStop         = events.ActionStop
	ActionCheckpoint   = events.ActionCheckpoint
	ActionPause        = events.ActionPause
	ActionUnPause      = events.ActionUnPause
	ActionAttach       = events.ActionAttach
	ActionDetach       = events.ActionDetach
	ActionResize       = events.ActionResize
	ActionUpdate       = events.ActionUpdate
	ActionRename       = events.ActionRename
	ActionKill         = events.ActionKill
	ActionDie          = events.ActionDie
	ActionOOM          = events.ActionOOM
	ActionDestroy      = events.ActionDestroy
	ActionRemove       = events.ActionRemove
	ActionCommit       = events.ActionCommit
	ActionTop          = events.ActionTop
	ActionCopy         = events.ActionCopy
	ActionArchivePath  = events.ActionArchivePath
	ActionExtractToDir = events.ActionExtractToDir
	ActionExport       = events.ActionExport
	ActionImport       = events.ActionImport
	ActionSave         = events.ActionSave
	ActionLoad         = events.ActionLoad
	ActionTag          = events.ActionTag
	ActionUnTag        = events.ActionUnTag
	ActionPush         = events.ActionPush
	ActionPull         = events.ActionPull
	ActionPrune        = events.ActionPrune
	ActionDelete       = events.ActionDelete
	ActionEnable       = events.ActionEnable
	ActionDisable      = events.ActionDisable
	ActionConnect      = events.ActionConnect
	ActionDisconnect   = events.ActionDisconnect
	ActionReload       = events.ActionReload
	ActionMount        = events.ActionMount
	ActionUnmount      = events.ActionUnmount

	// ActionExecCreate is the prefix used for exec_create events. These
	// event-actions are commonly followed by a colon and space (": "),
	// and the command that's defined for the exec, for example:
	//
	//	exec_create: /bin/sh -c 'echo hello'
	//
	// This is far from ideal; it's a compromise to allow filtering and
	// to preserve backward-compatibility.
	ActionExecCreate = events.ActionExecCreate
	// ActionExecStart is the prefix used for exec_create events. These
	// event-actions are commonly followed by a colon and space (": "),
	// and the command that's defined for the exec, for example:
	//
	//	exec_start: /bin/sh -c 'echo hello'
	//
	// This is far from ideal; it's a compromise to allow filtering and
	// to preserve backward-compatibility.
	ActionExecStart  = events.ActionExecStart
	ActionExecDie    = events.ActionExecDie
	ActionExecDetach = events.ActionExecDetach

	// ActionHealthStatus is the prefix to use for health_status events.
	//
	// Health-status events can either have a pre-defined status, in which
	// case the "health_status" action is followed by a colon, or can be
	// "free-form", in which case they're followed by the output of the
	// health-check output.
	//
	// This is far form ideal, and a compromise to allow filtering, and
	// to preserve backward-compatibility.
	ActionHealthStatus          = events.ActionHealthStatus
	ActionHealthStatusRunning   = events.ActionHealthStatusRunning
	ActionHealthStatusHealthy   = events.ActionHealthStatusHealthy
	ActionHealthStatusUnhealthy = events.ActionHealthStatusUnhealthy
)

// Actor describes something that generates events,
// like a container, or a network, or a volume.
// It has a defined name and a set of attributes.
// The container attributes are its labels, other actors
// can generate these attributes from other properties.
type Actor = events.Actor

// Message represents the information an event contains
type Message = events.Message

// ListOptions holds parameters to filter events with.
type ListOptions = events.ListOptions
