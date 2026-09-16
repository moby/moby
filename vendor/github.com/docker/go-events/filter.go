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

// Filter is the concrete implementation returned by [NewFilter].
//
// Deprecated: Filter should not be constructed directly. Use [NewFilter] instead.
type Filter = filter

type filter struct {
	dst     Sink
	matcher Matcher
	closed  bool
}

// NewFilter returns a new event sink that forwards only events accepted by
// matcher to dst.
//
// The returned Sink's methods are not safe for concurrent use.
func NewFilter(dst Sink, matcher Matcher) Sink {
	return &filter{dst: dst, matcher: matcher}
}

// Write an event to the filter.
func (f *filter) Write(event Event) error {
	if f.closed {
		return ErrSinkClosed
	}

	if f.matcher.Match(event) {
		return f.dst.Write(event)
	}

	return nil
}

// Close the filter and allow no more events to pass through.
func (f *filter) Close() error {
	// TODO(stevvooe): Not all sinks should have Close.
	if f.closed {
		return nil
	}

	f.closed = true
	return f.dst.Close()
}
