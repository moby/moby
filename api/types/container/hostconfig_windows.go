package container

import (
	"strings"
)

// IsDefault indicates whether container uses the default network stack.
func (n NetworkMode) IsDefault() bool {
	return n == "default"
}

// IsNone indicates whether container isn't using a network stack.
func (n NetworkMode) IsNone() bool {
	return n == "none"
}

// IsContainer indicates whether container uses a container network stack.
// Returns false as windows doesn't support this mode
func (n NetworkMode) IsContainer() bool {
	parts := strings.SplitN(string(n), ":", 2)
	return len(parts) > 1 && parts[0] == "container"
}

// IsBridge indicates whether container uses the bridge network stack
// in windows it is given the name NAT
func (n NetworkMode) IsBridge() bool {
	return n == "nat"
}

// IsHost indicates whether container uses the host network stack.
// returns false as this is not supported by windows
func (n NetworkMode) IsHost() bool {
	return false
}

// IsPrivate indicates whether container uses its private network stack.
func (n NetworkMode) IsPrivate() bool {
	return !(n.IsHost() || n.IsContainer())
}

// ConnectedContainer is the id of the container which network this container is connected to.
func (n NetworkMode) ConnectedContainer() string {
	parts := strings.SplitN(string(n), ":", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// IsUserDefined indicates user-created network
func (n NetworkMode) IsUserDefined() bool {
	return !n.IsDefault() && !n.IsNone() && !n.IsBridge() && !n.IsContainer()
}

// IsHyperV indicates the use of a Hyper-V partition for isolation
func (i Isolation) IsHyperV() bool {
	return strings.ToLower(string(i)) == "hyperv"
}

// IsProcess indicates the use of process isolation
func (i Isolation) IsProcess() bool {
	return strings.ToLower(string(i)) == "process"
}

// IsValid indicates if an isolation technology is valid
func (i Isolation) IsValid() bool {
	return i.IsDefault() || i.IsHyperV() || i.IsProcess()
}

// NetworkName returns the name of the network stack.
func (n NetworkMode) NetworkName() string {
	if n.IsDefault() {
		return "default"
	} else if n.IsBridge() {
		return "nat"
	} else if n.IsNone() {
		return "none"
	} else if n.IsContainer() {
		return "container"
	} else if n.IsUserDefined() {
		return n.UserDefined()
	}

	return ""
}

//UserDefined indicates user-created network
func (n NetworkMode) UserDefined() string {
	if n.IsUserDefined() {
		return string(n)
	}
	return ""
}
