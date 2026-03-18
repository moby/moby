package links

import (
	"cmp"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/moby/moby/api/types/network"
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
	Ports []network.Port
}

// EnvVars generates environment variables for the linked container
// for the Link with the given options.
func EnvVars(parentIP, childIP, name string, env []string, exposedPorts map[network.Port]struct{}) []string {
	return NewLink(parentIP, childIP, name, env, exposedPorts).ToEnv()
}

// NewLink initializes a new Link struct with the provided options.
func NewLink(parentIP, childIP, name string, env []string, exposedPorts map[network.Port]struct{}) *Link {
	ports := slices.Collect(maps.Keys(exposedPorts))

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

	env := make([]string, 0, 1+len(l.Ports)*4)
	var pStart, pEnd network.Port

	for i, p := range l.Ports {
		if i == 0 {
			pStart, pEnd = p, p
			env = append(env, fmt.Sprintf("%s_PORT=%s://%s:%d", alias, p.Proto(), l.ChildIP, p.Num()))
		}

		// These env-vars are produced for every port, regardless if they're part of a port-range.
		prefix := fmt.Sprintf("%s_PORT_%d_%s", alias, p.Num(), strings.ToUpper(string(p.Proto())))
		env = append(env, fmt.Sprintf("%s=%s://%s:%d", prefix, p.Proto(), l.ChildIP, p.Num()))
		env = append(env, fmt.Sprintf("%s_ADDR=%s", prefix, l.ChildIP))
		env = append(env, fmt.Sprintf("%s_PORT=%d", prefix, p.Num()))
		env = append(env, fmt.Sprintf("%s_PROTO=%s", prefix, p.Proto()))

		// Detect whether this port is part of a range (consecutive port number and same protocol).
		if p.Num() == pEnd.Num()+1 && p.Proto() == pEnd.Proto() {
			pEnd = p
			if i < len(l.Ports)-1 {
				continue
			}
		}

		if pEnd != pStart {
			prefix = fmt.Sprintf("%s_PORT_%d_%s", alias, pStart.Num(), strings.ToUpper(string(pStart.Proto())))
			env = append(env, fmt.Sprintf("%s_START=%s://%s:%d", prefix, pStart.Proto(), l.ChildIP, pStart.Num()))
			env = append(env, fmt.Sprintf("%s_PORT_START=%d", prefix, pStart.Num()))
			env = append(env, fmt.Sprintf("%s_END=%s://%s:%d", prefix, pEnd.Proto(), l.ChildIP, pEnd.Num()))
			env = append(env, fmt.Sprintf("%s_PORT_END=%d", prefix, pEnd.Num()))
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
func withTCPPriority(ip, jp network.Port) int {
	if ip.Proto() == jp.Proto() {
		return cmp.Compare(ip.Num(), jp.Num())
	}
	if ip.Proto() == network.TCP {
		return -1
	}
	if jp.Proto() == network.TCP {
		return 1
	}
	return cmp.Compare(ip.Proto(), jp.Proto())
}
