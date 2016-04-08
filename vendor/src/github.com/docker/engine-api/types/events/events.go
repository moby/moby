package events

const (
	// ContainerEventType is the event type that containers generate
	ContainerEventType = "container"
	// ImageEventType is the event type that images generate
	ImageEventType = "image"
	// VolumeEventType is the event type that volumes generate
	VolumeEventType = "volume"
	// NetworkEventType is the event type that networks generate
	NetworkEventType = "network"

	// Key attributes for used for the events above.

	// ContainerEventKey is the key used for container event attributes
	ContainerEventKey = "com.docker.container.name"
	// ContainerImageEventKey is the key used for container images event attributes
	ContainerImageEventKey = "com.docker.container.image"
	// ImageEventKey is the key used for image event attributes
	ImageEventKey = "com.docker.image.name"
	// VolumeEventKey is the key used for volume event attributes
	VolumeEventKey = "com.docker.volume.name"
	// NetworkEventKey is the key used for network event attributes
	NetworkEventKey = "com.docker.network.name"
	// NetworkEventTypeKey is the key used for network event attributes
	NetworkEventTypeKey = "com.docker.network.type"
)

// Actor describes something that generates events,
// like a container, or a network, or a volume.
// It has a defined name and a set or attributes.
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
	Status string `json:"status,omitempty"`
	ID     string `json:"id,omitempty"`
	From   string `json:"from,omitempty"`

	Type   string
	Action string
	Actor  Actor

	Time     int64 `json:"time,omitempty"`
	TimeNano int64 `json:"timeNano,omitempty"`
}
