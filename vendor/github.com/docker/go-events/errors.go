package events

import "fmt"

// ErrSinkClosed is returned if a write is issued to a sink that has been
// closed. If encountered, the error should be considered terminal and
// retries will not be successful.
var ErrSinkClosed = fmt.Errorf("events: sink closed")
