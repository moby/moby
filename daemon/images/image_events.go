package images // import "github.com/docker/docker/daemon/images"

import (
	"github.com/docker/docker/api/types/events"
)

// LogImageEvent generates an event related to an image with only the default attributes.
func (i *ImageService) LogImageEvent(imageID, refName, action string) {
	i.LogImageEventWithAttributes(imageID, refName, action, map[string]string{})
}

// LogImageEventWithAttributes generates an event related to an image with specific given attributes.
func (i *ImageService) LogImageEventWithAttributes(imageID, refName, action string, attributes map[string]string) {
	img, err := i.GetImage(nil, imageID, nil)
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

	i.eventsService.Log(action, events.ImageEventType, actor)
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
