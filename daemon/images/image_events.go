package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"encoding/json"

	"github.com/containerd/containerd/content"
	"github.com/docker/docker/api/types/events"
)

// LogImageEvent generates an event related to an image with only the default attributes.
func (i *ImageService) LogImageEvent(ctx context.Context, imageID, refName, action string) {
	// image has not been removed yet.
	// it could be missing if the event is `delete`.
	attributes, _ := i.getImageLabels(ctx, imageID)

	if attributes == nil {
		attributes = map[string]string{}
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

func (i *ImageService) getImageLabels(ctx context.Context, imageID string) (map[string]string, error) {
	img, err := i.GetImage(ctx, imageID)
	if err != nil {
		return nil, err
	}

	p, err := content.ReadBlob(ctx, i.client.ContentStore(), img)
	if err != nil {
		return nil, err
	}

	var config struct {
		Config struct {
			Labels map[string]string
		}
	}

	if err := json.Unmarshal(p, &config); err != nil {
		return nil, err
	}

	return config.Config.Labels, nil
}
