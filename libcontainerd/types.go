package libcontainerd

import "io"

// State constants used in state change reporting.
const (
	StateStart        = "start-container"
	StatePause        = "pause"
	StateResume       = "resume"
	StateExit         = "exit"
	StateRestart      = "restart"
	StateRestore      = "restore"
	StateStartProcess = "start-process"
	StateExitProcess  = "exit-process"
	StateOOM          = "oom" // fake state
	stateLive         = "live"
)

// CommonStateInfo contains the state info common to all platforms.
type CommonStateInfo struct { // FIXME: event?
	State     string
	Pid       uint32
	ExitCode  uint32
	ProcessID string
}

// Backend defines callbacks that the client of the library needs to implement.
type Backend interface {
	StateChanged(containerID string, state StateInfo) error
	AttachStreams(processFriendlyName string, io IOPipe) error
}

// Client provides access to containerd features.
type Client interface {
	Create(containerID string, spec Spec, options ...CreateOption) error
	Signal(containerID string, sig int) error
	SignalProcess(containerID string, processFriendlyName string, sig int) error
	AddProcess(containerID, processFriendlyName string, process Process) error
	Resize(containerID, processFriendlyName string, width, height int) error
	Pause(containerID string) error
	Resume(containerID string) error
	Restore(containerID string, options ...CreateOption) error
	Stats(containerID string) (*Stats, error)
	GetPidsForContainer(containerID string) ([]int, error)
	Summary(containerID string) ([]Summary, error)
	UpdateResources(containerID string, resources Resources) error
}

// CreateOption allows to configure parameters of container creation.
type CreateOption interface {
	Apply(interface{}) error
}

// IOPipe contains the stdio streams.
type IOPipe struct {
	Stdin    io.WriteCloser
	Stdout   io.Reader
	Stderr   io.Reader
	Terminal bool // Whether stderr is connected on Windows
}
