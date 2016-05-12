package events

import (
	"github.com/docker/docker/reference"
	"github.com/docker/engine-api/types/events"
	"github.com/docker/engine-api/types/filters"
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
	return ef.filter.ExactMatch("event", ev.Action) ||
		ef.filter.ExactMatch("type", ev.Type) ||
		ef.matchContainer(ev) ||
		ef.matchVolume(ev) ||
		ef.matchNetwork(ev) ||
		ef.matchImage(ev) ||
		ef.matchLabels(ev.Actor.Attributes)
}

func (ef *Filter) matchLabels(attributes map[string]string) bool {
	if !ef.filter.Include("label") {
		return false
	}
	return ef.filter.MatchKVList("label", attributes)
}

func (ef *Filter) matchContainer(ev events.Message) bool {
	return ef.fuzzyMatchName(ev, events.ContainerEventKey)
}

func (ef *Filter) matchVolume(ev events.Message) bool {
	return ef.fuzzyMatchName(ev, events.VolumeEventKey)
}

func (ef *Filter) matchNetwork(ev events.Message) bool {
	return ef.fuzzyMatchName(ev, events.NetworkEventKey)
}

func (ef *Filter) fuzzyMatchName(ev events.Message, eventType string) bool {
	return ef.filter.FuzzyMatch(eventType, ev.Actor.ID) ||
		ef.filter.FuzzyMatch(eventType, ev.Actor.Attributes[eventType])
}

// matchImage matches against both event.Actor.ID (for image events)
// and event.Actor.Attributes[events.ContainerImageEventKey] (for container events),
// so that any container that was created from an image will be included in the image
// events. Also compare both against the stripped repo name without any tags.
func (ef *Filter) matchImage(ev events.Message) bool {
	id := ev.Actor.ID
	nameAttr := events.ContainerImageEventKey
	var imageName string

	if ev.Type == events.ImageEventType {
		nameAttr = events.ImageEventKey
	}

	if n, ok := ev.Actor.Attributes[nameAttr]; ok {
		imageName = n
	}
	return ef.filter.ExactMatch(nameAttr, id) ||
		ef.filter.ExactMatch(nameAttr, imageName) ||
		ef.filter.ExactMatch(nameAttr, stripTag(id)) ||
		ef.filter.ExactMatch(nameAttr, stripTag(imageName))
}

func stripTag(image string) string {
	ref, err := reference.ParseNamed(image)
	if err != nil {
		return image
	}
	return ref.Name()
}
