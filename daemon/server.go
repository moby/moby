package daemon

type Server interface {
	IsRunning() bool // returns true if the server is currently in operation
}
