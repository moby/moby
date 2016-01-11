package daemon

import (
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/graphdb"
)

// linkIndex stores link relationships between containers, including their specified alias
// The alias is the name the parent uses to reference the child
type linkIndex struct {
	// idx maps a parent->alias->child relationship
	idx map[*container.Container]map[string]*container.Container
	// childIdx maps  child->parent->aliases
	childIdx map[*container.Container]map[*container.Container]map[string]struct{}
	mu       sync.Mutex
}

func newLinkIndex() *linkIndex {
	return &linkIndex{
		idx:      make(map[*container.Container]map[string]*container.Container),
		childIdx: make(map[*container.Container]map[*container.Container]map[string]struct{}),
	}
}

// link adds indexes for the passed in parent/child/alias relationships
func (l *linkIndex) link(parent, child *container.Container, alias string) {
	l.mu.Lock()

	if l.idx[parent] == nil {
		l.idx[parent] = make(map[string]*container.Container)
	}
	l.idx[parent][alias] = child
	if l.childIdx[child] == nil {
		l.childIdx[child] = make(map[*container.Container]map[string]struct{})
	}
	if l.childIdx[child][parent] == nil {
		l.childIdx[child][parent] = make(map[string]struct{})
	}
	l.childIdx[child][parent][alias] = struct{}{}

	l.mu.Unlock()
}

// unlink removes the requested alias for the given parent/child
func (l *linkIndex) unlink(alias string, child, parent *container.Container) {
	l.mu.Lock()
	delete(l.idx[parent], alias)
	delete(l.childIdx[child], parent)
	l.mu.Unlock()
}

// children maps all the aliases-> children for the passed in parent
// aliases here are the aliases the parent uses to refer to the child
func (l *linkIndex) children(parent *container.Container) map[string]*container.Container {
	l.mu.Lock()
	children := l.idx[parent]
	l.mu.Unlock()
	return children
}

// parents maps all the aliases->parent for the passed in child
// aliases here are the aliases the parents use to refer to the child
func (l *linkIndex) parents(child *container.Container) map[string]*container.Container {
	l.mu.Lock()

	parents := make(map[string]*container.Container)
	for parent, aliases := range l.childIdx[child] {
		for alias := range aliases {
			parents[alias] = parent
		}
	}

	l.mu.Unlock()
	return parents
}

// delete deletes all link relationships referencing this container
func (l *linkIndex) delete(container *container.Container) {
	l.mu.Lock()
	for _, child := range l.idx[container] {
		delete(l.childIdx[child], container)
	}
	delete(l.idx, container)
	delete(l.childIdx, container)
	l.mu.Unlock()
}

// migrateLegacySqliteLinks migrates sqlite links to use links from HostConfig
// when sqlite links were used, hostConfig.Links was set to nil
func (daemon *Daemon) migrateLegacySqliteLinks(db *graphdb.Database, container *container.Container) error {
	// if links is populated (or an empty slice), then this isn't using sqlite links and can be skipped
	if container.HostConfig == nil || container.HostConfig.Links != nil {
		return nil
	}

	logrus.Debugf("migrating legacy sqlite link info for container: %s", container.ID)

	fullName := container.Name
	if fullName[0] != '/' {
		fullName = "/" + fullName
	}

	// don't use a nil slice, this ensures that the check above will skip once the migration has completed
	links := []string{}
	children, err := db.Children(fullName, 0)
	if err != nil {
		if !strings.Contains(err.Error(), "Cannot find child for") {
			return err
		}
		// else continue... it's ok if we didn't find any children, it'll just be nil and we can continue the migration
	}

	for _, child := range children {
		c, err := daemon.GetContainer(child.Entity.ID())
		if err != nil {
			return err
		}

		links = append(links, c.Name+":"+child.Edge.Name)
	}

	container.HostConfig.Links = links
	return container.WriteHostConfig()
}
