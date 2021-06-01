package events

// Event marks items that can be sent as events.
type Event interface{}

// Sink accepts and sends events.
type Sink interface {
	// Write an event to the Sink. If no error is returned, the caller will
	// assume that all events have been committed to the sink. If an error is
	// received, the caller may retry sending the event.
	Write(event Event) error

	// Close the sink, possibly waiting for pending events to flush.
	Close() error
}
