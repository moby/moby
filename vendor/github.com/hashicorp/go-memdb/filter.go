package memdb

// FilterFunc is a function that takes the results of an iterator and returns
// whether the result should be filtered out.
type FilterFunc func(interface{}) bool

// FilterIterator is used to wrap a ResultIterator and apply a filter over it.
type FilterIterator struct {
	// filter is the filter function applied over the base iterator.
	filter FilterFunc

	// iter is the iterator that is being wrapped.
	iter ResultIterator
}

// NewFilterIterator wraps a ResultIterator. The filter function is applied
// to each value returned by a call to iter.Next.
//
// See the documentation for ResultIterator to understand the behaviour of the
// returned FilterIterator.
func NewFilterIterator(iter ResultIterator, filter FilterFunc) *FilterIterator {
	return &FilterIterator{
		filter: filter,
		iter:   iter,
	}
}

// WatchCh returns the watch channel of the wrapped iterator.
func (f *FilterIterator) WatchCh() <-chan struct{} { return f.iter.WatchCh() }

// Next returns the next non-filtered result from the wrapped iterator.
func (f *FilterIterator) Next() interface{} {
	for {
		if value := f.iter.Next(); value == nil || !f.filter(value) {
			return value
		}
	}
}
