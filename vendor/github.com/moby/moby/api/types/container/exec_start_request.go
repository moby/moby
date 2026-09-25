package container

// ExecStartRequest is a temp struct used by execStart
// Config fields is part of ExecConfig in runconfig package
type ExecStartRequest struct {
	// ExecStart will first check if it's detached
	Detach bool
	// Check if there's a tty
	Tty bool
	// Terminal size [height, width], unused if Tty == false
	ConsoleSize *[2]uint `json:",omitempty"`
}
