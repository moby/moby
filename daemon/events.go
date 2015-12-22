package daemon

import (
	"strings"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/container"
	"github.com/docker/libnetwork"
)

// LogContainerEvent generates an event related to a container.
func (daemon *Daemon) LogContainerEvent(container *container.Container, action string) {
	attributes := copyAttributes(container.Config.Labels)
	if container.Config.Image != "" {
		attributes["image"] = container.Config.Image
	}
	attributes["name"] = strings.TrimLeft(container.Name, "/")

	actor := events.Actor{
		ID:         container.ID,
		Attributes: attributes,
	}
	daemon.EventsService.Log(action, events.ContainerEventType, actor)
}

// LogImageEvent generates an event related to a container.
func (daemon *Daemon) LogImageEvent(imageID, refName, action string) {
	attributes := map[string]string{}
	img, err := daemon.GetImage(imageID)
	if err == nil && img.Config != nil {
		// image has not been removed yet.
		// it could be missing if the event is `delete`.
		attributes = copyAttributes(img.Config.Labels)
	}
	if refName != "" {
		attributes["name"] = refName
	}
	actor := events.Actor{
		ID:         imageID,
		Attributes: attributes,
	}

	daemon.EventsService.Log(action, events.ImageEventType, actor)
}

// LogVolumeEvent generates an event related to a volume.
func (daemon *Daemon) LogVolumeEvent(volumeID, action string, attributes map[string]string) {
	actor := events.Actor{
		ID:         volumeID,
		Attributes: attributes,
	}
	daemon.EventsService.Log(action, events.VolumeEventType, actor)
}

// LogNetworkEvent generates an event related to a network with only the default attributes.
func (daemon *Daemon) LogNetworkEvent(nw libnetwork.Network, action string) {
	daemon.LogNetworkEventWithAttributes(nw, action, map[string]string{})
}

// LogNetworkEventWithAttributes generates an event related to a network with specific given attributes.
func (daemon *Daemon) LogNetworkEventWithAttributes(nw libnetwork.Network, action string, attributes map[string]string) {
	attributes["name"] = nw.Name()
	attributes["type"] = nw.Type()
	daemon.logNetworkEventWithID(nw.ID(), action, attributes)
}

func (daemon *Daemon) logNetworkEventWithID(id, action string, attributes map[string]string) {
	actor := events.Actor{
		ID:         id,
		Attributes: attributes,
	}
	daemon.EventsService.Log(action, events.NetworkEventType, actor)
}

// copyAttributes guarantees that labels are not mutated by event triggers.
func copyAttributes(labels map[string]string) map[string]string {
	attributes := map[string]string{}
	if labels == nil {
		return attributes
	}
	for k, v := range labels {
		attributes[k] = v
	}
	return attributes
}
