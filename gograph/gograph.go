package gograph

import (
	"fmt"
	"path"
	"sync"
)

// Entity with a unique id and user defined value
type Entity struct {
	id    string
	Value interface{}
}

// An Edge connects two entities together
type Edge struct {
	EntityID string
	Name     string
	ParentID string
}

type Entities map[string]*Entity
type Edges []*Edge

type WalkFunc func(fullPath string, entity *Entity) error

// Graph database for storing entities and their relationships
type Database struct {
	entities Entities
	edges    Edges
	mux      sync.Mutex

	rootID string
}

// Create a new graph database initialized with a root entity
func NewDatabase(rootPath, rootId string) (*Database, error) {
	db := &Database{Entities{}, Edges{}, sync.Mutex{}, rootId}
	e := &Entity{
		id: rootId,
	}
	db.entities[rootId] = e

	edge := &Edge{
		EntityID: rootId,
		Name:     "/",
	}
	db.edges = append(db.edges, edge)

	return db, nil
}

// Set the entity id for a given path
func (db *Database) Set(fullPath, id string) (*Entity, error) {
	db.mux.Lock()
	defer db.mux.Unlock()

	e, exists := db.entities[id]
	if !exists {
		e = &Entity{
			id: id,
		}
		db.entities[id] = e
	}

	parentPath, name := splitPath(fullPath)
	if err := db.setEdge(parentPath, name, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (db *Database) setEdge(parentPath, name string, e *Entity) error {
	parent := db.Get(parentPath)
	if parent == nil {
		return fmt.Errorf("Parent does not exist for path: %s", parentPath)
	}
	if parent.id == e.id {
		return fmt.Errorf("Cannot set self as child")
	}

	edge := &Edge{
		ParentID: parent.id,
		EntityID: e.id,
		Name:     name,
	}
	if db.edges.Exists(parent.id, name) {
		return fmt.Errorf("Relationship already exists for %s/%s", parentPath, name)
	}
	db.edges = append(db.edges, edge)
	return nil
}

// Return the root "/" entity for the database
func (db *Database) RootEntity() *Entity {
	return db.entities[db.rootID]
}

// Return the entity for a given path
func (db *Database) Get(name string) *Entity {
	e := db.RootEntity()
	// We always know the root name so return it if
	// it is requested
	if name == "/" {
		return e
	}

	parts := split(name)
	for i := 1; i < len(parts); i++ {
		p := parts[i]

		next := db.child(e, p)
		if next == nil {
			return nil
		}
		e = next

	}
	return e
}

// List all entities by from the name
// The key will be the full path of the entity
func (db *Database) List(name string, depth int) Entities {
	out := Entities{}
	for c := range db.children(name, depth) {
		out[c.FullPath] = c.Entity
	}
	return out
}

func (db *Database) Walk(name string, walkFunc WalkFunc, depth int) error {
	for c := range db.children(name, depth) {
		if err := walkFunc(c.FullPath, c.Entity); err != nil {
			return err
		}
	}
	return nil
}

// Return the refrence count for a specified id
func (db *Database) Refs(id string) int {
	return len(db.RefPaths(id))
}

// Return all the id's path references
func (db *Database) RefPaths(id string) Edges {
	refs := db.edges.Search(func(e *Edge) bool {
		return e.EntityID == id
	})
	return refs
}

// Delete the reference to an entity at a given path
func (db *Database) Delete(name string) error {
	if name == "/" {
		return fmt.Errorf("Cannot delete root entity")
	}
	db.mux.Lock()
	defer db.mux.Unlock()

	parentPath, n := splitPath(name)
	parent := db.Get(parentPath)
	if parent == nil {
		return fmt.Errorf("Cannot find parent for %s", parentPath)
	}
	edge, i := db.edges.Get(parent.id, n)
	if edge == nil {
		return fmt.Errorf("Edge does not exist at %s", name)
	}
	db.deleteEdgeAtIndex(i)

	return nil
}

func (db *Database) deleteEdgeAtIndex(i int) {
	db.edges[len(db.edges)-1], db.edges[i], db.edges = nil, db.edges[len(db.edges)-1], db.edges[:len(db.edges)-1]
}

// Remove the entity with the specified id
// Walk the graph to make sure all references to the entity
// are removed and return the number of references removed
func (db *Database) Purge(id string) (int, error) {
	db.mux.Lock()
	defer db.mux.Unlock()

	getIndex := func(e *Edge) int {
		for i, edge := range db.edges {
			if edge.EntityID == e.EntityID &&
				edge.Name == e.Name &&
				edge.ParentID == e.ParentID {
				return i
			}
		}
		return -1
	}

	refsToDelete := db.RefPaths(id)
	for i, e := range refsToDelete {
		index := getIndex(e)
		if index == -1 {
			return i + 1, fmt.Errorf("Cannot find index for %s %s", e.ParentID, e.Name)
		}
		db.deleteEdgeAtIndex(index)
	}
	return len(refsToDelete), nil
}

// Rename an edge for a given path
func (db *Database) Rename(currentName, newName string) error {
	parentPath, name := splitPath(currentName)
	newParentPath, newEdgeName := splitPath(newName)

	if parentPath != newParentPath {
		return fmt.Errorf("Cannot rename when root paths do not match %s != %s", parentPath, newParentPath)
	}

	db.mux.Lock()
	defer db.mux.Unlock()

	parent := db.Get(parentPath)
	if parent == nil {
		return fmt.Errorf("Cannot locate parent for %s", currentName)
	}
	edge, _ := db.edges.Get(parent.id, name)
	if edge == nil {
		return fmt.Errorf("Cannot locate edge for %s %s", parent.id, name)
	}
	edge.Name = newEdgeName

	return nil
}

type WalkMeta struct {
	Parent   *Entity
	Entity   *Entity
	FullPath string
	Edge     *Edge
}

func (db *Database) children(name string, depth int) <-chan WalkMeta {
	out := make(chan WalkMeta)
	e := db.Get(name)

	if e == nil {
		close(out)
		return out
	}

	go func() {
		for _, edge := range db.edges {
			if edge.ParentID == e.id {
				child := db.entities[edge.EntityID]

				meta := WalkMeta{
					Parent:   e,
					Entity:   child,
					FullPath: path.Join(name, edge.Name),
					Edge:     edge,
				}
				out <- meta
				if depth == 0 {
					continue
				}
				nDepth := depth
				if depth != -1 {
					nDepth -= 1
				}
				sc := db.children(meta.FullPath, nDepth)
				for c := range sc {
					out <- c
				}
			}
		}
		close(out)
	}()
	return out
}

// Return the entity based on the parent path and name
func (db *Database) child(parent *Entity, name string) *Entity {
	edge, _ := db.edges.Get(parent.id, name)
	if edge == nil {
		return nil
	}
	return db.entities[edge.EntityID]
}

// Return the id used to reference this entity
func (e *Entity) ID() string {
	return e.id
}

// Return the paths sorted by depth
func (e Entities) Paths() []string {
	out := make([]string, len(e))
	var i int
	for k := range e {
		out[i] = k
		i++
	}
	sortByDepth(out)

	return out
}

// Checks if an edge with the specified parent id and name exist in the slice
func (e Edges) Exists(parendId, name string) bool {
	edge, _ := e.Get(parendId, name)
	return edge != nil
}

// Returns the edge and index in the slice with the specified parent id and name
func (e Edges) Get(parentId, name string) (*Edge, int) {
	for i, edge := range e {
		if edge.ParentID == parentId && edge.Name == name {
			return edge, i
		}
	}
	return nil, -1
}

func (e Edges) Search(predicate func(edge *Edge) bool) Edges {
	out := Edges{}
	for _, edge := range e {
		if predicate(edge) {
			out = append(out, edge)
		}
	}
	return out
}
