package image

import (
	"context"

	"github.com/docker/docker/api/types/events"
	imagetypes "github.com/docker/docker/api/types/image"
	daemonevents "github.com/docker/docker/daemon/events"
)

// EventLogger produces daemon events for image lifecycle
type EventLogger struct {
	Events   *daemonevents.Events
	GetImage func(ctx context.Context, refOrID string, options imagetypes.GetImageOpts) (*Image, error)
}

// LogImageEvent generates an event related to an image.
func (e EventLogger) LogImageEvent(imageID, refName, action string) {
	attributes := map[string]string{}
	img, err := e.GetImage(nil, imageID, imagetypes.GetImageOpts{})
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

	e.Events.Log(action, events.ImageEventType, actor)
}

// copyAttributes guarantees that labels are not mutated by event triggers.
func copyAttributes(attributes, labels map[string]string) {
	if labels == nil {
		return
	}
	for k, v := range labels {
		attributes[k] = v
	}
}
