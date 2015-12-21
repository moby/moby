package events

import (
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/reference"
)

// Filter can filter out docker events from a stream
type Filter struct {
	filter filters.Args
}

// NewFilter creates a new Filter
func NewFilter(filter filters.Args) *Filter {
	return &Filter{filter: filter}
}

// Include returns true when the event ev is included by the filters
func (ef *Filter) Include(ev events.Message) bool {
	if ev.Type != events.ContainerEventType && ev.Type != events.ImageEventType {
		return false
	}
	return ef.filter.ExactMatch("event", ev.Action) &&
		ef.matchContainer(ev) &&
		ef.isImageIncluded(ev) &&
		ef.isLabelFieldIncluded(ev.Actor.Attributes)
}

func (ef *Filter) isLabelFieldIncluded(attributes map[string]string) bool {
	if !ef.filter.Include("label") {
		return true
	}
	return ef.filter.MatchKVList("label", attributes)
}

func (ef *Filter) matchContainer(ev events.Message) bool {
	return ef.filter.FuzzyMatch("container", ev.Actor.ID) ||
		ef.filter.FuzzyMatch("container", ev.Actor.Attributes["name"])
}

// The image filter will be matched against both event.ID (for image events)
// and event.From (for container events), so that any container that was created
// from an image will be included in the image events. Also compare both
// against the stripped repo name without any tags.
func (ef *Filter) isImageIncluded(ev events.Message) bool {
	id := ev.ID
	var imageName string
	if n, ok := ev.Actor.Attributes["image"]; ok {
		imageName = n
	}
	return ef.filter.ExactMatch("image", id) ||
		ef.filter.ExactMatch("image", imageName) ||
		ef.filter.ExactMatch("image", stripTag(id)) ||
		ef.filter.ExactMatch("image", stripTag(imageName))
}

func stripTag(image string) string {
	ref, err := reference.ParseNamed(image)
	if err != nil {
		return image
	}
	return ref.Name()
}
