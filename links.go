package docker

import (
	"fmt"
	"github.com/dotcloud/docker/iptables"
	"github.com/dotcloud/docker/utils"
	"strings"
)

// A Link represents a connection between two containers
// for a specific port on a specific bridge interface
type Link struct {
	FromID          string
	ToID            string
	FromIP          string
	ToIP            string
	BridgeInterface string
	Alias           string
	FromEnvironment []string
	ports           []Port
	isEnabled       bool
}

type LinkRepository struct {
	links map[string]*Link
}

func (r *LinkRepository) NewLink(to, from *Container, bridgeInterface string, alias string) (*Link, error) {
	if !from.State.Running {
		return nil, fmt.Errorf("Cannot link to a non running container: %s AS %s", from.ID, alias)
	}
	ports := make([]Port, len(from.Config.ExposedPorts))
	var i int
	for k := range from.Config.ExposedPorts {
		ports[i] = k
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
		ports:           ports,
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
	// Load exposed ports into the environment
	for _, p := range l.ports {
		env = append(env, fmt.Sprintf("%s_%s_ADDR=%s://%s:%s", strings.ToUpper(l.Alias), p.Port(), p.Proto(), l.FromIP, p.Port()))
	}

	if l.FromEnvironment != nil {
		for _, v := range l.FromEnvironment {
			parts := strings.Split(v, "=")
			if len(parts) < 2 {
				continue
			}
			env = append(env, fmt.Sprintf("%s_ENV_%s=%s", strings.ToUpper(l.Alias), parts[0], parts[1]))
		}
	}
	return env
}

func (l *Link) Enable() error {
	if err := l.toggle("-I"); err != nil {
		return err
	}
	l.isEnabled = true
	return nil
}

func (l *Link) Disable() {
	// We do not care about erros here because the link may not
	// exist in iptables
	l.toggle("-D")

	l.isEnabled = false
}

func (l *Link) toggle(action string) error {
	for _, p := range l.ports {
		if err := iptables.Raw(action, "FORWARD",
			"-i", l.BridgeInterface, "-o", l.BridgeInterface,
			"-p", p.Proto(),
			"-s", l.ToIP,
			"--dport", p.Port(),
			"-d", l.FromIP,
			"-j", "ACCEPT"); err != nil {
			return err
		}

		if err := iptables.Raw(action, "FORWARD",
			"-i", l.BridgeInterface, "-o", l.BridgeInterface,
			"-p", p.Proto(),
			"-s", l.FromIP,
			"--sport", p.Port(),
			"-d", l.ToIP,
			"-j", "ACCEPT"); err != nil {
			return err
		}
	}
	return nil
}

func NewLinkRepository(root string) (*LinkRepository, error) {
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
