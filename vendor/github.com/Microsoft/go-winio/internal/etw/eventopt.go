package etw

import (
	"golang.org/x/sys/windows"
)

type eventOptions struct {
	descriptor        *EventDescriptor
	activityID        *windows.GUID
	relatedActivityID *windows.GUID
	tags              uint32
}

// EventOpt defines the option function type that can be passed to
// Provider.WriteEvent to specify general event options, such as level and
// keyword.
type EventOpt func(options *eventOptions)

// WithEventOpts returns the variadic arguments as a single slice.
func WithEventOpts(opts ...EventOpt) []EventOpt {
	return opts
}

// WithLevel specifies the level of the event to be written.
func WithLevel(level Level) EventOpt {
	return func(options *eventOptions) {
		options.descriptor.Level = level
	}
}

// WithKeyword specifies the keywords of the event to be written. Multiple uses
// of this option are OR'd together.
func WithKeyword(keyword uint64) EventOpt {
	return func(options *eventOptions) {
		options.descriptor.Keyword |= keyword
	}
}

func WithChannel(channel Channel) EventOpt {
	return func(options *eventOptions) {
		options.descriptor.Channel = channel
	}
}

// WithTags specifies the tags of the event to be written. Tags is a 28-bit
// value (top 4 bits are ignored) which are interpreted by the event consumer.
func WithTags(newTags uint32) EventOpt {
	return func(options *eventOptions) {
		options.tags |= newTags
	}
}

func WithActivityID(activityID *windows.GUID) EventOpt {
	return func(options *eventOptions) {
		options.activityID = activityID
	}
}

func WithRelatedActivityID(activityID *windows.GUID) EventOpt {
	return func(options *eventOptions) {
		options.relatedActivityID = activityID
	}
}
