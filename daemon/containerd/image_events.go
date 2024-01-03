package containerd

import (
	"context"

	"github.com/containerd/containerd/images"
	"github.com/docker/docker/api/types/events"
	imagetypes "github.com/docker/docker/api/types/image"
)

// LogImageEvent generates an event related to an image with only the default attributes.
func (i *ImageService) LogImageEvent(imageID, refName string, action events.Action) {
	ctx := context.TODO()
	attributes := map[string]string{}

	img, err := i.GetImage(ctx, imageID, imagetypes.GetImageOpts{})
	if err == nil && img.Config != nil {
		// image has not been removed yet.
		// it could be missing if the event is `delete`.
		copyAttributes(attributes, img.Config.Labels)
	}
	if refName != "" {
		attributes["name"] = refName
	}
	i.eventsService.Log(action, events.ImageEventType, events.Actor{
		ID:         imageID,
		Attributes: attributes,
	})
}

// logImageEvent generates an event related to an image with only name attribute.
func (i *ImageService) logImageEvent(img images.Image, refName string, action events.Action) {
	attributes := map[string]string{}
	if refName != "" {
		attributes["name"] = refName
	}
	i.eventsService.Log(action, events.ImageEventType, events.Actor{
		ID:         img.Target.Digest.String(),
		Attributes: attributes,
	})
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
