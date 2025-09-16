package links

import (
	"cmp"
	"fmt"
	"path"
	"slices"
	"strings"

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
	Ports []container.PortProto // TODO(thaJeztah): can we use []string here, or do we need the features of nat.Port?
}

// EnvVars generates environment variables for the linked container
// for the Link with the given options.
func EnvVars(parentIP, childIP, name string, env []string, exposedPorts map[container.PortProto]struct{}) []string {
	return NewLink(parentIP, childIP, name, env, exposedPorts).ToEnv()
}

// NewLink initializes a new Link struct with the provided options.
func NewLink(parentIP, childIP, name string, env []string, exposedPorts map[container.PortProto]struct{}) *Link {
	ports := make([]container.PortProto, 0, len(exposedPorts))
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
	slices.SortFunc(l.Ports, withTCPPriority)

	var pStart, pEnd portIsh
	env := make([]string, 0, 1+len(l.Ports)*4)
	for i, p := range l.Ports {
		if i == 0 {
			pStart, pEnd = p, p
			env = append(env, fmt.Sprintf("%s_PORT=%s://%s:%s", alias, p.Proto(), l.ChildIP, p.Port()))
		}

		// These env-vars are produced for every port, regardless if they're
		// part of a port-range.
		portNum, err := p.Int()
		if err != nil {
			continue
		}
		prefix := fmt.Sprintf("%s_PORT_%d_%s", alias, portNum, strings.ToUpper(p.Proto()))
		env = append(env, fmt.Sprintf("%s=%s://%s:%d", prefix, p.Proto(), l.ChildIP, portNum))
		env = append(env, fmt.Sprintf("%s_ADDR=%s", prefix, l.ChildIP))
		env = append(env, fmt.Sprintf("%s_PORT=%d", prefix, portNum))
		env = append(env, fmt.Sprintf("%s_PROTO=%s", prefix, p.Proto()))

		// Detect whether this port is part of a range (consecutive port number
		// and same protocol).
		portEndNum, err := pEnd.Int()
		if err != nil {
			continue
		}
		if portNum == portEndNum+1 && strings.EqualFold(p.Proto(), pStart.Proto()) {
			pEnd = p
			if i < len(l.Ports)-1 {
				continue
			}
		}

		if pEnd != pStart {
			portStartNum, err := pStart.Int()
			if err != nil {
				continue
			}
			prefix = fmt.Sprintf("%s_PORT_%d_%s", alias, portStartNum, strings.ToUpper(pStart.Proto()))
			env = append(env, fmt.Sprintf("%s_START=%s://%s:%d", prefix, pStart.Proto(), l.ChildIP, portStartNum))
			env = append(env, fmt.Sprintf("%s_PORT_START=%d", prefix, portStartNum))
			env = append(env, fmt.Sprintf("%s_END=%s://%s:%d", prefix, pEnd.Proto(), l.ChildIP, portEndNum))
			env = append(env, fmt.Sprintf("%s_PORT_END=%d", prefix, portEndNum))
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

// FIXME(thaJeztah): update [nat.Sort] signature to accept an interface instead of only nat.Port as concrete type.
type portIsh interface {
	Proto() string
	Int() (int, error)
}

// withTCPPriority prioritizes ports using TCP over other protocols before
// comparing port-number and protocol.
func withTCPPriority[T portIsh](ip, jp T) int {
	if strings.EqualFold(ip.Proto(), jp.Proto()) {
		i, _ := ip.Int()
		j, _ := jp.Int()
		return cmp.Compare(i, j)
	}
	var c int
	if strings.EqualFold(ip.Proto(), "tcp") {
		c--
	}
	if strings.EqualFold(jp.Proto(), "tcp") {
		c++
	}
	if c != 0 {
		return c
	}
	return cmp.Compare(strings.ToLower(ip.Proto()), strings.ToLower(jp.Proto()))
}
