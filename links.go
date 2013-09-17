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
	Port            Port
	Alias           string
	FromEnvironment []string
	isEnabled       bool
}

type LinkRepository struct {
	links map[string]*Link
}

func (r *LinkRepository) NewLink(to, from *Container, bridgeInterface string, p Port, alias string) (*Link, error) {
	if !from.State.Running {
		return nil, fmt.Errorf("Cannot link to a non running container: %s AS %s", from.ID, alias)
	}
	if !from.Exposes(p) {
		return nil, fmt.Errorf("Cannot link to %s because %s is not exposed", from.ID, p)
	}
	l := &Link{
		FromID:          utils.TruncateID(from.ID),
		ToID:            utils.TruncateID(to.ID),
		BridgeInterface: bridgeInterface,
		Alias:           alias,
		Port:            p,
		FromIP:          from.NetworkSettings.IPAddress,
		ToIP:            to.NetworkSettings.IPAddress,
		FromEnvironment: from.Config.Env,
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
	env := []string{fmt.Sprintf("%s_ADDR=%s://%s:%s", strings.ToUpper(l.Alias), l.Port.Proto(), l.FromIP, l.Port.Port())}
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
	if err := iptables.Raw(action, "FORWARD",
		"-i", l.BridgeInterface, "-o", l.BridgeInterface,
		"-p", l.Port.Proto(),
		"-s", l.ToIP,
		"--dport", l.Port.Port(),
		"-d", l.FromIP,
		"-j", "ACCEPT"); err != nil {
		return err
	}

	if err := iptables.Raw(action, "FORWARD",
		"-i", l.BridgeInterface, "-o", l.BridgeInterface,
		"-p", l.Port.Proto(),
		"-s", l.FromIP,
		"--sport", l.Port.Port(),
		"-d", l.ToIP,
		"-j", "ACCEPT"); err != nil {
		return err
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
