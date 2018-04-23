package exec

// WaitCondition is a type used to specify an exec state for which
// to wait.
type WaitCondition string

// Possible WaitCondition Values.
//
// WaitConditionRunning is used to wait for the exec to be running.
//
// WaitConditionExited is used to wait for the exec to exit.
const (
	WaitConditionRunning WaitCondition = "running"
	WaitConditionExited                = "exit"
)
