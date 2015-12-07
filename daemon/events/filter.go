package events

import (
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/parsers/filters"
)

// Filter can filter out docker events from a stream
type Filter struct {
	filter       filters.Args
	getLabels    func(id string) map[string]string
	updateFilter func(f *filters.Args)
}

// NewFilter creates a new Filter
func NewFilter(filter filters.Args, getLabels func(id string) map[string]string, updateFilter func(f *filters.Args)) *Filter {
	return &Filter{filter: filter, getLabels: getLabels, updateFilter: updateFilter}
}

// Include returns true when the event ev is included by the filters
func (ef *Filter) Include(ev *jsonmessage.JSONMessage) bool {
	return ef.filter.ExactMatch("event", ev.Status) &&
		ef.isContainerIDIncluded(ev.ID) &&
		ef.isImageIncluded(ev.ID, ev.From) &&
		ef.isLabelFieldIncluded(ev.ID)
}

func (ef *Filter) isLabelFieldIncluded(id string) bool {
	if !ef.filter.Include("label") {
		return true
	}
	return ef.filter.MatchKVList("label", ef.getLabels(id))
}

func (ef *Filter) isContainerIDIncluded(id string) bool {
	if !ef.filter.Include("container") {
		return true
	}
	ef.updateFilter(&ef.filter)
	return ef.filter.ExactMatch("container", id)
}

// The image filter will be matched against both event.ID (for image events)
// and event.From (for container events), so that any container that was created
// from an image will be included in the image events. Also compare both
// against the stripped repo name without any tags.
func (ef *Filter) isImageIncluded(eventID string, eventFrom string) bool {
	return ef.filter.ExactMatch("image", eventID) ||
		ef.filter.ExactMatch("image", eventFrom) ||
		ef.filter.ExactMatch("image", stripTag(eventID)) ||
		ef.filter.ExactMatch("image", stripTag(eventFrom))
}

func stripTag(image string) string {
	ref, err := reference.ParseNamed(image)
	if err != nil {
		return image
	}
	return ref.Name()
}
