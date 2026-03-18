package wasip1

// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-eventtype-enumu8
const (
	// EventTypeClock is the timeout event named "name".
	EventTypeClock = iota
	// EventTypeFdRead is the data available event named "fd_read".
	EventTypeFdRead
	// EventTypeFdWrite is the capacity available event named "fd_write".
	EventTypeFdWrite
)

const (
	PollOneoffName = "poll_oneoff"
)
