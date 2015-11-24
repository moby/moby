package events

import (
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/parsers/filters"
)

// Filter can filter out docker events from a stream
type Filter struct {
	filter    filters.Args
	getLabels func(id string) map[string]string
}

// NewFilter creates a new Filter
func NewFilter(filter filters.Args, getLabels func(id string) map[string]string) *Filter {
	return &Filter{filter: filter, getLabels: getLabels}
}

// Include returns true when the event ev is included by the filters
func (ef *Filter) Include(ev *jsonmessage.JSONMessage) bool {
	return isFieldIncluded(ev.Status, ef.filter["event"]) &&
		isFieldIncluded(ev.ID, ef.filter["container"]) &&
		ef.isImageIncluded(ev.ID, ev.From) &&
		ef.isLabelFieldIncluded(ev.ID)
}

func (ef *Filter) isLabelFieldIncluded(id string) bool {
	if _, ok := ef.filter["label"]; !ok {
		return true
	}
	return ef.filter.MatchKVList("label", ef.getLabels(id))
}

// The image filter will be matched against both event.ID (for image events)
// and event.From (for container events), so that any container that was created
// from an image will be included in the image events. Also compare both
// against the stripped repo name without any tags.
func (ef *Filter) isImageIncluded(eventID string, eventFrom string) bool {
	stripTag := func(image string) string {
		ref, err := reference.ParseNamed(image)
		if err != nil {
			return image
		}
		return ref.Name()
	}

	return isFieldIncluded(eventID, ef.filter["image"]) ||
		isFieldIncluded(eventFrom, ef.filter["image"]) ||
		isFieldIncluded(stripTag(eventID), ef.filter["image"]) ||
		isFieldIncluded(stripTag(eventFrom), ef.filter["image"])
}

func isFieldIncluded(field string, filter []string) bool {
	if len(field) == 0 {
		return true
	}
	if len(filter) == 0 {
		return true
	}
	for _, v := range filter {
		if v == field {
			return true
		}
	}
	return false
}
