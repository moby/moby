package events // import "github.com/docker/docker/api/types/events"

// Type is used for event-types.
type Type = string

// List of known event types.
const (
	BuilderEventType   Type = "builder"   // BuilderEventType is the event type that the builder generates.
	ConfigEventType    Type = "config"    // ConfigEventType is the event type that configs generate.
	ContainerEventType Type = "container" // ContainerEventType is the event type that containers generate.
	DaemonEventType    Type = "daemon"    // DaemonEventType is the event type that daemon generate.
	ImageEventType     Type = "image"     // ImageEventType is the event type that images generate.
	NetworkEventType   Type = "network"   // NetworkEventType is the event type that networks generate.
	NodeEventType      Type = "node"      // NodeEventType is the event type that nodes generate.
	PluginEventType    Type = "plugin"    // PluginEventType is the event type that plugins generate.
	SecretEventType    Type = "secret"    // SecretEventType is the event type that secrets generate.
	ServiceEventType   Type = "service"   // ServiceEventType is the event type that services generate.
	VolumeEventType    Type = "volume"    // VolumeEventType is the event type that volumes generate.
)

// Actor describes something that generates events,
// like a container, or a network, or a volume.
// It has a defined name and a set of attributes.
// The container attributes are its labels, other actors
// can generate these attributes from other properties.
type Actor struct {
	ID         string
	Attributes map[string]string
}

// Message represents the information an event contains
type Message struct {
	// Deprecated information from JSONMessage.
	// With data only in container events.
	Status string `json:"status,omitempty"` // Deprecated: use Action instead.
	ID     string `json:"id,omitempty"`     // Deprecated: use Actor.ID instead.
	From   string `json:"from,omitempty"`   // Deprecated: use Actor.Attributes["image"] instead.

	Type   Type
	Action string
	Actor  Actor
	// Engine events are local scope. Cluster events are swarm scope.
	Scope string `json:"scope,omitempty"`

	Time     int64 `json:"time,omitempty"`
	TimeNano int64 `json:"timeNano,omitempty"`
}
