/*
The container index is a set of simple maps used to handle:
* naming containers
* aliasing containers
* managing the parent/child relationship of containers
* Persisting this information to disk safely.

It is a replacement for pkg/graphdb which will be retired hopefully soon.
*/
package daemon

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/pkg/graphdb"
	"github.com/docker/docker/pkg/truncindex"
)

const (
	containerIndexFileName = "container_index.json"
	tickInterval           = time.Second
)

type containerIndex struct {
	// child name -> parent name -> id
	NameToParents map[string]map[string]string

	// parent name -> child name -> id
	NameToChildren map[string]map[string]string

	// container name -> alias -> id
	Aliases map[string]map[string]string

	// container name -> child names -> id
	AliasChildMap map[string]map[string]string

	// container name -> id
	NameToID map[string]string

	// container id -> name
	IDToName map[string]string

	// container id -> container
	idToContainer map[string]*Container

	daemon     *Daemon
	indexPath  string // fully qualified containerIndexFileName
	truncIndex *truncindex.TruncIndex
	mutex      *sync.Mutex
	syncTicker *time.Ticker
}

// migrate the linkgraph graphdb to the new containerIndex format.
func migrateIndex(root string, daemon *Daemon) (*containerIndex, error) {
	graphdbPath := filepath.Join(root, "linkgraph.db")
	c := emptyContainerIndex(root, daemon)

	// bail only if we can find the file but there's problems otherwise loading
	// the graphdb. If the file does not exist, we have migrated successfully.
	if _, err := os.Stat(graphdbPath); err != nil {
		return nil, err
	}

	graph, err := graphdb.NewSqliteConn(graphdbPath)
	if err != nil {
		return nil, err
	}

	// this is extremely hairy code. What it's doing here is mining the graphdb
	// for names, and aliases. In the event it finds a name, it will attempt to
	// load the container. Docker reloads these containers on startup, so as long
	// as this is done first (as it is currently) there should be no issue.
	// Otherwise, certain runtime things will not be attached like
	// BroadcastWriter and Daemon. In the event it finds a link, it will set up a
	// parent/child relationship, and also relate the last portion (the alias) as
	// the container's alias.

	// used for building the parent/child mappings
	relationships := map[string]string{}
	aliases := map[string]string{}

	for p, e := range graph.List("/", 3) {
		parts := strings.Split(p, "/")
		parts = parts[1:] // remove leading slash

		switch len(parts) {
		case 1: // only a name
			cont, err := c.daemon.Load(e.ID())
			if err != nil {
				continue
			}
			c.NameToID[parts[0]] = e.ID()
			c.IDToName[e.ID()] = parts[0]
			if cont.Name[0] == '/' {
				cont.Name = cont.Name[1:]
				cont.ToDisk()
			}
			c.idToContainer[e.ID()] = cont
		case 2: // parent -> alias link.
			relationships[parts[0]] = e.ID()
			aliases[e.ID()] = parts[1]
		}
	}

	for parent, childID := range relationships {
		child := c.idToContainer[childID]
		if child == nil {
			continue
		}

		c.NameToParents[child.Name] = map[string]string{parent: c.NameToID[parent]}
		c.NameToChildren[parent] = map[string]string{child.Name: childID}
		c.addAlias(parent, child.Name, aliases[childID])
	}

	if err = c.ToDisk(); err == nil {
		graph.Close()
		os.Remove(graphdbPath)
	}

	return c, err
}

func getIndexFilename(root string) string {
	return filepath.Join(root, containerIndexFileName)
}

func (c *containerIndex) startSync() {
	for _ = range c.syncTicker.C {
		c.ToDisk()
	}
}

// This is what is used to load the container index. It also happens to do
// several other things via function dispatch:
// * migrates graphdb
// * creates an empty container index if needed
func containerIndexFromDisk(root string, daemon *Daemon) (*containerIndex, error) {
	var c *containerIndex

	if _, err := os.Stat(getIndexFilename(root)); err != nil {
		c, err = migrateIndex(root, daemon)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	if c == nil {
		c = emptyContainerIndex(root, daemon)
	}

	content, err := ioutil.ReadFile(c.indexPath)

	if err != nil {
		// file does not exist, or there was a permission error. Start from scratch.
		go c.startSync()
		return c, nil
	}

	if err := json.Unmarshal(content, c); err != nil {
		return nil, err
	}

	for id := range c.IDToName {
		c.truncIndex.Add(id)
	}

	go c.startSync()
	return c, nil
}

func emptyContainerIndex(root string, daemon *Daemon) *containerIndex {
	return &containerIndex{
		NameToParents:  map[string]map[string]string{},
		NameToChildren: map[string]map[string]string{},
		Aliases:        map[string]map[string]string{},
		AliasChildMap:  map[string]map[string]string{},
		NameToID:       map[string]string{},
		IDToName:       map[string]string{},

		idToContainer: map[string]*Container{},
		daemon:        daemon,
		indexPath:     getIndexFilename(root),
		truncIndex:    truncindex.NewTruncIndex([]string{}),
		mutex:         new(sync.Mutex),
		syncTicker:    time.NewTicker(tickInterval),
	}
}

func (c *containerIndex) lock() {
	c.mutex.Lock()
}

func (c *containerIndex) unlock() {
	c.mutex.Unlock()
}

func (c *containerIndex) ToDisk() error {
	c.lock()
	defer c.unlock()

	f, err := ioutil.TempFile("", "container_index")
	if err != nil {
		return err
	}

	// the file is closed later if there are no errors. this is a noop at that point.
	defer f.Close()

	content, err := json.Marshal(c)
	if err != nil {
		return err
	}

	if _, err := f.Write(content); err != nil {
		return err
	}

	f.Close()

	if err := os.Rename(f.Name(), c.indexPath); err != nil {
		return err
	}

	return nil
}

func (c *containerIndex) Add(cont *Container) error {
	c.lock()
	c.add(cont)
	c.unlock()

	return nil
}

func (c *containerIndex) add(cont *Container) {
	// this is to fix a migration issue where the containers would be assigned a name starting with /
	if cont.Name[0] == '/' {
		cont.Name = cont.Name[1:]
	}

	if _, ok := c.NameToParents[cont.Name]; !ok {
		c.NameToParents[cont.Name] = map[string]string{}
	}

	if _, ok := c.NameToChildren[cont.Name]; !ok {
		c.NameToChildren[cont.Name] = map[string]string{}
	}

	c.NameToID[cont.Name] = cont.ID
	c.IDToName[cont.ID] = cont.Name
	c.truncIndex.Add(cont.ID)
	c.idToContainer[cont.ID] = cont
}

func (c *containerIndex) Delete(cont *Container) {
	c.lock()
	c.doDelete(cont)
	c.unlock()
}

func (c *containerIndex) Unlink(cont *Container) {
	c.lock()
	c.removeParents(cont)
	c.unlock()
}

func (c *containerIndex) removeParents(cont *Container) {
	for name := range c.NameToParents[cont.Name] {
		delete(c.NameToChildren[name], cont.Name)
		delete(c.Aliases[name], cont.Name)
		delete(c.AliasChildMap[name], cont.Name)
	}

	delete(c.NameToParents, cont.Name)
}

func (c *containerIndex) doDelete(cont *Container) {
	delete(c.NameToParents, cont.Name)
	delete(c.NameToChildren, cont.Name)
	delete(c.Aliases, cont.Name)
	delete(c.AliasChildMap, cont.Name)
	delete(c.NameToID, cont.Name)
	delete(c.IDToName, cont.ID)
	delete(c.idToContainer, cont.ID)
}

func (c *containerIndex) Link(parent *Container, child *Container) {
	c.lock()
	c.cleanChildParent(parent, child)
	c.addChild(parent, child)
	c.addParent(parent, child)
	c.unlock()
}

func (c *containerIndex) cleanChildParent(parent, child *Container) {
	delete(c.NameToChildren[parent.Name], child.Name)
	delete(c.NameToParents[child.Name], parent.Name)
}

func (c *containerIndex) addChild(parent, child *Container) {
	if c.NameToChildren[parent.Name] == nil {
		c.NameToChildren[parent.Name] = map[string]string{}
	}

	c.NameToChildren[parent.Name][child.Name] = child.ID
}

func (c *containerIndex) addParent(parent, child *Container) {
	if c.NameToParents[child.Name] == nil {
		c.NameToParents[child.Name] = map[string]string{}
	}

	c.NameToParents[child.Name][parent.Name] = parent.ID
}

func (c *containerIndex) ParentsByName(name string) map[string]*Container {
	c.lock()

	result := map[string]*Container{}

	for name, id := range c.NameToParents[name] {
		result[name] = c.idToContainer[id]
	}

	c.unlock()
	return result
}

func (c *containerIndex) ChildrenByName(name string) map[string]*Container {
	c.lock()

	result := map[string]*Container{}

	for name, id := range c.NameToChildren[name] {
		result[name] = c.idToContainer[id]
	}

	c.unlock()

	return result
}

func (c *containerIndex) SetAlias(parent, child *Container, alias string) error {
	c.lock()
	defer c.unlock()

	// locks are already used in AliasInUse and AddAlias

	if c.aliasInUse(parent, child, alias) {
		return fmt.Errorf("Alias %s cannot be set for container ID %s, parent %s: name conflict", alias, child.ID, parent.ID)
	}

	if err := c.addAlias(parent.Name, child.Name, alias); err != nil {
		return err
	}

	return nil
}

func (c *containerIndex) aliasInUse(parent, child *Container, alias string) bool {
	// lock is acquired in caller
	if _, ok := c.Aliases[parent.Name]; ok {
		_, aliasok := c.Aliases[parent.Name][child.Name]
		_, childok := c.AliasChildMap[parent.Name][alias]
		return aliasok || childok
	}

	return false
}

func (c *containerIndex) GetByID(id string) (*Container, error) {
	c.lock()
	defer c.unlock()

	if _, ok := c.IDToName[id]; ok {
		if cont, ok := c.idToContainer[id]; ok {
			return cont, nil
		}
	}

	return nil, fmt.Errorf("Container for %s cannot be located", id)
}

func (c *containerIndex) GetByTruncName(name string) (*Container, error) {
	c.lock()
	defer c.unlock()

	id, err := c.truncIndex.Get(name)
	if err == nil {
		if cont, ok := c.idToContainer[id]; ok {
			return cont, nil
		}
	}

	return nil, fmt.Errorf("Container for %s cannot be located", name)
}

func (c *containerIndex) GetByName(name string) (*Container, error) {
	c.lock()
	defer c.unlock()

	if id, ok := c.NameToID[name]; ok {
		if cont, ok := c.idToContainer[id]; ok {
			return cont, nil
		}
	}

	return nil, fmt.Errorf("Container for %s cannot be located", name)
}

func (c *containerIndex) GetNameForID(id string) string {
	c.lock()
	defer c.unlock()
	return c.IDToName[id]
}

// This function can be used *before* containers are loaded to map names to
// identifiers. This is needed because of (*Daemon).ensureName()
func (c *containerIndex) AssignNameForID(name, id string) error {
	if c.NameInUse(name) {
		return fmt.Errorf("Name %s is already in use", name)
	}

	c.lock()
	c.IDToName[id] = name
	c.unlock()

	return nil
}

func (c *containerIndex) NameInUse(name string) bool {
	c.lock()
	_, nameok := c.NameToID[name]
	c.unlock()

	return nameok
}

func (c *containerIndex) List() []*Container {
	containers := new(History)
	c.lock()
	for _, id := range c.NameToID {
		cont, ok := c.idToContainer[id]
		if !ok {
			continue
		}

		containers.Add(cont)
	}
	c.unlock()
	containers.Sort()
	return *containers
}

// Pulls this structure apart: /parent/child/alias into two containers and an alias
func (c *containerIndex) DeconstructPath(path string) (*Container, *Container, string, error) {
	c.lock()
	defer c.unlock()

	parts := strings.Split(path, "/")

	if parts[0] == "" {
		parts = parts[1:]
	}

	if len(parts) < 2 {
		return nil, nil, "", fmt.Errorf("Invalid path (missing entities)")
	}

	var parent, child *Container
	var alias string

	if id, ok := c.NameToID[parts[0]]; ok {
		parent = c.idToContainer[id]
		if id, ok := c.NameToID[parts[1]]; ok {
			child = c.idToContainer[id]
		}
	}

	if parent == nil {
		return nil, nil, "", fmt.Errorf("Could not locate parent container %s", parts[0])
	}

	if child == nil {
		return nil, nil, "", fmt.Errorf("Could not locate child container %s", parts[1])
	}

	if len(parts) > 2 && parts[2] != "" {
		alias = parts[2]
	} else {
		alias = child.Name
	}

	return parent, child, alias, nil
}

func (c *containerIndex) ChildFor(parent *Container, alias string) *Container {
	c.lock()
	defer c.unlock()

	if childname, ok := c.AliasChildMap[parent.Name][alias]; ok {
		if childid, ok := c.NameToID[childname]; ok {
			if child, ok := c.idToContainer[childid]; ok {
				return child
			}
		}
	}

	return nil
}

func (c *containerIndex) AliasFor(parent, child *Container) string {
	c.lock()
	defer c.unlock()

	alias := c.Aliases[parent.Name][child.Name]
	if alias != "" {
		return alias
	}

	return child.Name
}

func (c *containerIndex) addAlias(parent, child, alias string) error {
	// lock is acquired in caller
	if _, ok := c.Aliases[parent]; !ok {
		c.Aliases[parent] = map[string]string{}
	}

	if _, ok := c.AliasChildMap[parent]; !ok {
		c.AliasChildMap[parent] = map[string]string{}
	}

	if _, ok := c.Aliases[parent][child]; ok {
		return fmt.Errorf("Alias already set for parent %s, child %s", parent, child)
	}

	if _, ok := c.AliasChildMap[parent][alias]; ok {
		return fmt.Errorf("Alias already set for parent %s, alias %s", parent, alias)
	}

	c.Aliases[parent][child] = alias
	c.AliasChildMap[parent][alias] = child

	return nil
}
