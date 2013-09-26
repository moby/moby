package docker

import (
	"fmt"
	"github.com/dotcloud/docker/iptables"
	"github.com/dotcloud/docker/utils"
	"strings"
)

type Link struct {
	FromID          string
	ToID            string
	FromIP          string
	ToIP            string
	BridgeInterface string
	Alias           string
	FromEnvironment []string
	Ports           []Port
	IsEnabled       bool
}

type LinkRepository struct {
	links map[string]*Link
}

func (r *LinkRepository) NewLink(to, from *Container, bridgeInterface string, alias string) (*Link, error) {
	if to.ID == from.ID {
		return nil, fmt.Errorf("Cannot link to self: %s == %s", to.ID, from.ID)
	}
	if !from.State.Running {
		return nil, fmt.Errorf("Cannot link to a non running container: %s AS %s", from.ID, alias)
	}
	ports := make([]Port, len(from.Config.ExposedPorts))
	var i int
	for p := range from.Config.ExposedPorts {
		ports[i] = p
		i++
	}
	l := &Link{
		FromID:          utils.TruncateID(from.ID),
		ToID:            utils.TruncateID(to.ID),
		BridgeInterface: bridgeInterface,
		Alias:           alias,
		FromIP:          from.NetworkSettings.IPAddress,
		ToIP:            to.NetworkSettings.IPAddress,
		FromEnvironment: from.Config.Env,
		Ports:           ports,
	}
	if err := r.registerLink(l); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *Link) ID() string {
	return fmt.Sprintf("%s:%s", l.ToID, l.Alias)
}

func (l *Link) ToEnv() []string {
	env := []string{}

	if p := l.getDefaultPort(); p != nil {
		env = append(env, fmt.Sprintf("%s_PORT=%s://%s:%s", l.Alias, p.Proto(), l.FromIP, p.Port()))
	}

	// Load exposed ports into the environment
	for _, p := range l.Ports {
		env = append(env, fmt.Sprintf("%s_PORT_%s_%s=%s://%s:%s", l.Alias, p.Port(), p.Proto(), p.Proto(), l.FromIP, p.Port()))
	}

	// Load the linked container's ID into the environment
	env = append(env, fmt.Sprintf("%s_ID=%s", l.Alias, l.FromID))

	if l.FromEnvironment != nil {
		for _, v := range l.FromEnvironment {
			parts := strings.Split(v, "=")
			if len(parts) != 2 {
				continue
			}
			// Ignore a few variables that are added during docker build
			if parts[0] == "HOME" || parts[0] == "PATH" {
				continue
			}
			env = append(env, fmt.Sprintf("%s_ENV_%s=%s", l.Alias, parts[0], parts[1]))
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
	// We do not care about erros here because the link may not
	// exist in iptables
	l.toggle("-D", true)

	l.IsEnabled = false
}

func (l *Link) toggle(action string, ignoreErrors bool) error {
	for _, p := range l.Ports {
		if err := iptables.Raw(action, "FORWARD",
			"-i", l.BridgeInterface, "-o", l.BridgeInterface,
			"-p", p.Proto(),
			"-s", l.ToIP,
			"--dport", p.Port(),
			"-d", l.FromIP,
			"-j", "ACCEPT"); !ignoreErrors && err != nil {
			return err
		}

		if err := iptables.Raw(action, "FORWARD",
			"-i", l.BridgeInterface, "-o", l.BridgeInterface,
			"-p", p.Proto(),
			"-s", l.FromIP,
			"--sport", p.Port(),
			"-d", l.ToIP,
			"-j", "ACCEPT"); !ignoreErrors && err != nil {
			return err
		}
	}
	return nil
}

func NewLinkRepository() (*LinkRepository, error) {
	r := &LinkRepository{make(map[string]*Link)}
	return r, nil
}

// Return all links for a container
func (l *LinkRepository) Get(c *Container) []*Link {
	id := utils.TruncateID(c.ID)
	out := []*Link{}
	for _, link := range l.links {
		if link.ToID == id || link.FromID == id {
			out = append(out, link)
		}
	}
	return out
}

// Return all links in the repository
func (l *LinkRepository) GetAll() []*Link {
	out := make([]*Link, len(l.links))
	var i int
	for _, link := range l.links {
		out[i] = link
		i++
	}
	return out
}

// Get a link based on the link's ID
func (l *LinkRepository) GetById(id string) *Link {
	return l.links[id]
}

// Create a new link with a unique alias
func (l *LinkRepository) registerLink(link *Link) error {
	if _, exists := l.links[link.ID()]; exists {
		return fmt.Errorf("A link for %s already exists", link.ID())
	}
	utils.Debugf("Registering link: %s", link.ID())
	l.links[link.ID()] = link

	return nil
}

// Disable and remote the link from the repository
func (l *LinkRepository) removeLink(link *Link) error {
	link.Disable()

	utils.Debugf("Removing link: %s", link.ID())
	delete(l.links, link.ID())
	return nil
}
