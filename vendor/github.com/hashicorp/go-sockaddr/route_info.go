package sockaddr

// RouteInterface specifies an interface for obtaining memoized route table and
// network information from a given OS.
type RouteInterface interface {
	// GetDefaultInterfaceName returns the name of the interface that has a
	// default route or an error and an empty string if a problem was
	// encountered.
	GetDefaultInterfaceName() (string, error)
}

// VisitCommands visits each command used by the platform-specific RouteInfo
// implementation.
func (ri routeInfo) VisitCommands(fn func(name string, cmd []string)) {
	for k, v := range ri.cmds {
		cmds := append([]string(nil), v...)
		fn(k, cmds)
	}
}
