package links

import (
	"fmt"
	"path"
	"strings"

	"github.com/docker/go-connections/nat"
	"github.com/moby/moby/api/types/container"
)

// Link struct holds information about parent/child linked container
type Link struct {
	// Parent container IP address
	ParentIP string
	// Child container IP address
	ChildIP string
	// Link name
	Name string
	// Child environments variables
	ChildEnvironment []string
	// Child exposed ports
	Ports []container.PortRangeProto // TODO(thaJeztah): can we use []string here, or do we need the features of nat.Port?
}

// EnvVars generates environment variables for the linked container
// for the Link with the given options.
func EnvVars(parentIP, childIP, name string, env []string, exposedPorts map[container.PortRangeProto]struct{}) []string {
	return NewLink(parentIP, childIP, name, env, exposedPorts).ToEnv()
}

// NewLink initializes a new Link struct with the provided options.
func NewLink(parentIP, childIP, name string, env []string, exposedPorts map[container.PortRangeProto]struct{}) *Link {
	ports := make([]container.PortRangeProto, 0, len(exposedPorts))
	for p := range exposedPorts {
		ports = append(ports, p)
	}

	return &Link{
		Name:             name,
		ChildIP:          childIP,
		ParentIP:         parentIP,
		ChildEnvironment: env,
		Ports:            ports,
	}
}

// ToEnv creates a string's slice containing child container information in
// the form of environment variables which will be later exported on container
// startup.
func (l *Link) ToEnv() []string {
	_, n := path.Split(l.Name)
	alias := strings.ReplaceAll(strings.ToUpper(n), "-", "_")

	// sort the ports so that we can bulk the continuous ports together
	nat.Sort(l.Ports, withTCPPriority)

	var pStart, pEnd container.PortRangeProto
	env := make([]string, 0, 1+len(l.Ports)*4)
	for i, p := range l.Ports {
		if i == 0 {
			pStart, pEnd = p, p
			env = append(env, fmt.Sprintf("%s_PORT=%s://%s:%s", alias, p.Proto(), l.ChildIP, p.Port()))
		}

		// These env-vars are produced for every port, regardless if they're
		// part of a port-range.
		prefix := fmt.Sprintf("%s_PORT_%s_%s", alias, p.Port(), strings.ToUpper(p.Proto()))
		env = append(env, fmt.Sprintf("%s=%s://%s:%s", prefix, p.Proto(), l.ChildIP, p.Port()))
		env = append(env, fmt.Sprintf("%s_ADDR=%s", prefix, l.ChildIP))
		env = append(env, fmt.Sprintf("%s_PORT=%s", prefix, p.Port()))
		env = append(env, fmt.Sprintf("%s_PROTO=%s", prefix, p.Proto()))

		// Detect whether this port is part of a range (consecutive port number
		// and same protocol).
		if p.Int() == pEnd.Int()+1 && strings.EqualFold(p.Proto(), pStart.Proto()) {
			pEnd = p
			if i < len(l.Ports)-1 {
				continue
			}
		}

		if pEnd != pStart {
			prefix = fmt.Sprintf("%s_PORT_%s_%s", alias, pStart.Port(), strings.ToUpper(pStart.Proto()))
			env = append(env, fmt.Sprintf("%s_START=%s://%s:%s", prefix, pStart.Proto(), l.ChildIP, pStart.Port()))
			env = append(env, fmt.Sprintf("%s_PORT_START=%s", prefix, pStart.Port()))
			env = append(env, fmt.Sprintf("%s_END=%s://%s:%s", prefix, pEnd.Proto(), l.ChildIP, pEnd.Port()))
			env = append(env, fmt.Sprintf("%s_PORT_END=%s", prefix, pEnd.Port()))
		}

		// Reset for next range (if any)
		pStart, pEnd = p, p
	}

	// Load the linked container's name into the environment
	env = append(env, fmt.Sprintf("%s_NAME=%s", alias, l.Name))

	if l.ChildEnvironment != nil {
		for _, v := range l.ChildEnvironment {
			name, val, ok := strings.Cut(v, "=")
			if !ok {
				continue
			}
			// Ignore a few variables that are added during docker build (and not really relevant to linked containers)
			if name == "HOME" || name == "PATH" {
				continue
			}
			env = append(env, fmt.Sprintf("%s_ENV_%s=%s", alias, name, val))
		}
	}
	return env
}

// withTCPPriority prioritizes ports using TCP over other protocols before
// comparing port-number and protocol.
func withTCPPriority(ip, jp container.PortRangeProto) bool {
	if strings.EqualFold(ip.Proto(), jp.Proto()) {
		return ip.Int() < jp.Int()
	}

	if strings.EqualFold(ip.Proto(), "tcp") {
		return true
	}
	if strings.EqualFold(jp.Proto(), "tcp") {
		return false
	}

	return strings.ToLower(ip.Proto()) < strings.ToLower(jp.Proto())
}
