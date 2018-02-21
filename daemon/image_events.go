package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/api/types/events"
)

// LogImageEvent generates an event related to an image with only the default attributes.
func (daemon *Daemon) LogImageEvent(imageID, refName, action string) {
	daemon.LogImageEventWithAttributes(imageID, refName, action, map[string]string{})
}

// LogImageEventWithAttributes generates an event related to an image with specific given attributes.
func (daemon *Daemon) LogImageEventWithAttributes(imageID, refName, action string, attributes map[string]string) {
	img, err := daemon.GetImage(imageID)
	if err == nil && img.Config != nil {
		// image has not been removed yet.
		// it could be missing if the event is `delete`.
		copyAttributes(attributes, img.Config.Labels)
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
