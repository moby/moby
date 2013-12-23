package docker

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/iptables"
	"path"
	"strings"
)

type Link struct {
	ParentIP         string
	ChildIP          string
	Name             string
	BridgeInterface  string
	ChildEnvironment []string
	Ports            []Port
	IsEnabled        bool
}

func NewLink(parent, child *Container, name, bridgeInterface string) (*Link, error) {
	if parent.ID == child.ID {
		return nil, fmt.Errorf("Cannot link to self: %s == %s", parent.ID, child.ID)
	}
	if !child.State.IsRunning() {
		return nil, fmt.Errorf("Cannot link to a non running container: %s AS %s", child.Name, name)
	}

	ports := make([]Port, len(child.Config.ExposedPorts))
	var i int
	for p := range child.Config.ExposedPorts {
		ports[i] = p
		i++
	}

	l := &Link{
		BridgeInterface:  bridgeInterface,
		Name:             name,
		ChildIP:          child.NetworkSettings.IPAddress,
		ParentIP:         parent.NetworkSettings.IPAddress,
		ChildEnvironment: child.Config.Env,
		Ports:            ports,
	}
	return l, nil

}

func (l *Link) Alias() string {
	_, alias := path.Split(l.Name)
	return alias
}

func (l *Link) ToEnv() []string {
	env := []string{}
	alias := strings.ToUpper(l.Alias())

	if p := l.getDefaultPort(); p != nil {
		env = append(env, fmt.Sprintf("%s_PORT=%s://%s:%s", alias, p.Proto(), l.ChildIP, p.Port()))
	}

	// Load exposed ports into the environment
	for _, p := range l.Ports {
		env = append(env, fmt.Sprintf("%s_PORT_%s_%s=%s://%s:%s", alias, p.Port(), strings.ToUpper(p.Proto()), p.Proto(), l.ChildIP, p.Port()))
		env = append(env, fmt.Sprintf("%s_PORT_%s_%s_ADDR=%s", alias, p.Port(), strings.ToUpper(p.Proto()), l.ChildIP))
		env = append(env, fmt.Sprintf("%s_PORT_%s_%s_PORT=%s", alias, p.Port(), strings.ToUpper(p.Proto()), p.Port()))
		env = append(env, fmt.Sprintf("%s_PORT_%s_%s_PROTO=%s", alias, p.Port(), strings.ToUpper(p.Proto()), p.Proto()))
	}

	// Load the linked container's name into the environment
	env = append(env, fmt.Sprintf("%s_NAME=%s", alias, l.Name))

	if l.ChildEnvironment != nil {
		for _, v := range l.ChildEnvironment {
			parts := strings.Split(v, "=")
			if len(parts) != 2 {
				continue
			}
			// Ignore a few variables that are added during docker build
			if parts[0] == "HOME" || parts[0] == "PATH" {
				continue
			}
			env = append(env, fmt.Sprintf("%s_ENV_%s=%s", alias, parts[0], parts[1]))
		}
	}
	return env
}

// Default port rules
func (l *Link) getDefaultPort() *Port {
	var p Port
	i := len(l.Ports)

	if i == 0 {
		return nil
	} else if i > 1 {
		sortPorts(l.Ports, func(ip, jp Port) bool {
			// If the two ports have the same number, tcp takes priority
			// Sort in desc order
			return ip.Int() < jp.Int() || (ip.Int() == jp.Int() && strings.ToLower(ip.Proto()) == "tcp")
		})
	}
	p = l.Ports[0]
	return &p
}

func (l *Link) Enable() error {
	if err := l.toggle("-I", false); err != nil {
		return err
	}
	l.IsEnabled = true
	return nil
}

func (l *Link) Disable() {
	// We do not care about errors here because the link may not
	// exist in iptables
	l.toggle("-D", true)

	l.IsEnabled = false
}

func (l *Link) toggle(action string, ignoreErrors bool) error {
	for _, p := range l.Ports {
		if output, err := iptables.Raw(action, "FORWARD",
			"-i", l.BridgeInterface, "-o", l.BridgeInterface,
			"-p", p.Proto(),
			"-s", l.ParentIP,
			"--dport", p.Port(),
			"-d", l.ChildIP,
			"-j", "ACCEPT"); !ignoreErrors && err != nil {
			return err
		} else if len(output) != 0 {
			return fmt.Errorf("Error toggle iptables forward: %s", output)
		}

		if output, err := iptables.Raw(action, "FORWARD",
			"-i", l.BridgeInterface, "-o", l.BridgeInterface,
			"-p", p.Proto(),
			"-s", l.ChildIP,
			"--sport", p.Port(),
			"-d", l.ParentIP,
			"-j", "ACCEPT"); !ignoreErrors && err != nil {
			return err
		} else if len(output) != 0 {
			return fmt.Errorf("Error toggle iptables forward: %s", output)
		}
	}
	return nil
}
