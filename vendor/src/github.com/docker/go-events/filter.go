package events

// Matcher matches events.
type Matcher interface {
	Match(event Event) bool
}

// MatcherFunc implements matcher with just a function.
type MatcherFunc func(event Event) bool

// Match calls the wrapped function.
func (fn MatcherFunc) Match(event Event) bool {
	return fn(event)
}

// Filter provides an event sink that sends only events that are accepted by a
// Matcher. No methods on filter are goroutine safe.
type Filter struct {
	dst     Sink
	matcher Matcher
	closed  bool
}

// NewFilter returns a new filter that will send to events to dst that return
// true for Matcher.
func NewFilter(dst Sink, matcher Matcher) Sink {
	return &Filter{dst: dst, matcher: matcher}
}

// Write an event to the filter.
func (f *Filter) Write(event Event) error {
	if f.closed {
		return ErrSinkClosed
	}

	if f.matcher.Match(event) {
		return f.dst.Write(event)
	}

	return nil
}

// Close the filter and allow no more events to pass through.
func (f *Filter) Close() error {
	// TODO(stevvooe): Not all sinks should have Close.
	if f.closed {
		return nil
	}

	f.closed = true
	return f.dst.Close()
}
