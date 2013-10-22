package gograph

import (
	"database/sql"
	"fmt"
	"path"
)

const (
	createEntityTable = `
    CREATE TABLE IF NOT EXISTS entity (
        id text NOT NULL PRIMARY KEY
    );`

	createEdgeTable = `
    CREATE TABLE IF NOT EXISTS edge (
        "entity_id" text NOT NULL,
        "parent_id" text NULL,
        "name" text NOT NULL,
        CONSTRAINT "parent_fk" FOREIGN KEY ("parent_id") REFERENCES "entity" ("id"),
        CONSTRAINT "entity_fk" FOREIGN KEY ("entity_id") REFERENCES "entity" ("id")
        );
    `

	createEdgeIndices = `
    CREATE UNIQUE INDEX IF NOT EXISTS "name_parent_ix" ON "edge" (parent_id, name);
    `
)

// Entity with a unique id
type Entity struct {
	id string
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
	conn *sql.DB
}

// Create a new graph database initialized with a root entity
func NewDatabase(conn *sql.DB, init bool) (*Database, error) {
	if conn == nil {
		return nil, fmt.Errorf("Database connection cannot be nil")
	}
	db := &Database{conn}

	if init {
		if _, err := conn.Exec(createEntityTable); err != nil {
			return nil, err
		}
		if _, err := conn.Exec(createEdgeTable); err != nil {
			return nil, err
		}
		if _, err := conn.Exec(createEdgeIndices); err != nil {
			return nil, err
		}

		rollback := func() {
			conn.Exec("ROLLBACK")
		}

		// Create root entities
		if _, err := conn.Exec("BEGIN"); err != nil {
			return nil, err
		}
		if _, err := conn.Exec("INSERT INTO entity (id) VALUES (?);", "0"); err != nil {
			rollback()
			return nil, err
		}

		if _, err := conn.Exec("INSERT INTO edge (entity_id, name) VALUES(?,?);", "0", "/"); err != nil {
			rollback()
			return nil, err
		}

		if _, err := conn.Exec("COMMIT"); err != nil {
			return nil, err
		}
	}
	return db, nil
}

// Close the underlying connection to the database
func (db *Database) Close() error {
	return db.conn.Close()
}

// Set the entity id for a given path
func (db *Database) Set(fullPath, id string) (*Entity, error) {
	// FIXME: is rollback implicit when closing the connection?
	rollback := func() {
		db.conn.Exec("ROLLBACK")
	}
	// FIXME: use exclusive transactions to avoid race conditions
	if _, err := db.conn.Exec("BEGIN"); err != nil {
		return nil, err
	}
	var entityId string
	if err := db.conn.QueryRow("SELECT id FROM entity WHERE id = ?;", id).Scan(&entityId); err != nil {
		if err == sql.ErrNoRows {
			if _, err := db.conn.Exec("INSERT INTO entity (id) VALUES(?);", id); err != nil {
				rollback()
				return nil, err
			}
		} else {
			rollback()
			return nil, err
		}
	}
	e := &Entity{id}

	parentPath, name := splitPath(fullPath)
	if err := db.setEdge(parentPath, name, e); err != nil {
		rollback()
		return nil, err
	}

	if _, err := db.conn.Exec("COMMIT"); err != nil {
		return nil, err
	}
	return e, nil
}

func (db *Database) setEdge(parentPath, name string, e *Entity) error {
	parent, err := db.get(parentPath)
	if err != nil {
		return err
	}
	if parent.id == e.id {
		return fmt.Errorf("Cannot set self as child")
	}

	if _, err := db.conn.Exec("INSERT INTO edge (parent_id, name, entity_id) VALUES (?,?,?);", parent.id, name, e.id); err != nil {
		return err
	}
	return nil
}

// Return the root "/" entity for the database
func (db *Database) RootEntity() *Entity {
	return &Entity{
		id: "0",
	}
}

// Return the entity for a given path
func (db *Database) Get(name string) *Entity {
	e, err := db.get(name)
	if err != nil {
		return nil
	}
	return e
}

func (db *Database) get(name string) (*Entity, error) {
	e := db.RootEntity()
	// We always know the root name so return it if
	// it is requested
	if name == "/" {
		return e, nil
	}

	parts := split(name)
	for i := 1; i < len(parts); i++ {
		p := parts[i]

		next := db.child(e, p)
		if next == nil {
			return nil, fmt.Errorf("Cannot find child")
		}
		e = next
	}
	return e, nil

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
	var count int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM edge WHERE entity_id = ?;", id).Scan(&count); err != nil {
		return 0
	}
	return count
}

// Return all the id's path references
func (db *Database) RefPaths(id string) Edges {
	refs := Edges{}

	rows, err := db.conn.Query("SELECT name, parent_id FROM edge WHERE entity_id = ?;", id)
	if err != nil {
		return refs
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var parentId string
		if err := rows.Scan(&name, &parentId); err != nil {
			return refs
		}
		refs = append(refs, &Edge{
			EntityID: id,
			Name:     name,
			ParentID: parentId,
		})
	}
	return refs
}

// Delete the reference to an entity at a given path
func (db *Database) Delete(name string) error {
	if name == "/" {
		return fmt.Errorf("Cannot delete root entity")
	}

	parentPath, n := splitPath(name)
	parent, err := db.get(parentPath)
	if err != nil {
		return err
	}

	if _, err := db.conn.Exec("DELETE FROM edge WHERE parent_id = ? AND name LIKE ?;", parent.id, n+"%"); err != nil {
		return err
	}
	return nil
}

// Remove the entity with the specified id
// Walk the graph to make sure all references to the entity
// are removed and return the number of references removed
func (db *Database) Purge(id string) (int, error) {
	rollback := func() {
		db.conn.Exec("ROLLBACK")
	}

	if _, err := db.conn.Exec("BEGIN"); err != nil {
		return -1, err
	}

	// Delete all edges
	rows, err := db.conn.Exec("DELETE FROM edge WHERE entity_id = ?;", id)
	if err != nil {
		rollback()
		return -1, err
	}

	changes, err := rows.RowsAffected()
	if err != nil {
		return -1, err
	}

	// Delete entity
	if _, err := db.conn.Exec("DELETE FROM entity where id = ?;", id); err != nil {
		rollback()
		return -1, err
	}

	if _, err := db.conn.Exec("COMMIT"); err != nil {
		return -1, err
	}
	return int(changes), nil
}

// Rename an edge for a given path
func (db *Database) Rename(currentName, newName string) error {
	parentPath, name := splitPath(currentName)
	newParentPath, newEdgeName := splitPath(newName)

	if parentPath != newParentPath {
		return fmt.Errorf("Cannot rename when root paths do not match %s != %s", parentPath, newParentPath)
	}

	parent, err := db.get(parentPath)
	if err != nil {
		return err
	}

	rows, err := db.conn.Exec("UPDATE edge SET name = ? WHERE parent_id = ? AND name LIKE ?;", newEdgeName, parent.id, name+"%")
	if err != nil {
		return err
	}
	i, err := rows.RowsAffected()
	if err != nil {
		return err
	}
	if i == 0 {
		return fmt.Errorf("Cannot locate edge for %s %s", parent.id, name)
	}
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
	e, err := db.get(name)
	if err != nil {
		close(out)
		return out
	}

	go func() {
		rows, err := db.conn.Query("SELECT entity_id, name FROM edge where parent_id = ?;", e.id)
		if err != nil {
			close(out)
		}
		defer rows.Close()

		for rows.Next() {
			var entityId, entityName string
			if err := rows.Scan(&entityId, &entityName); err != nil {
				// Log error
				continue
			}
			child := &Entity{entityId}
			edge := &Edge{
				ParentID: e.id,
				Name:     entityName,
				EntityID: child.id,
			}

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
		close(out)
	}()
	return out
}

// Return the entity based on the parent path and name
func (db *Database) child(parent *Entity, name string) *Entity {
	var id string
	if err := db.conn.QueryRow("SELECT entity_id FROM edge WHERE parent_id = ? AND name LIKE ?;", parent.id, name+"%").Scan(&id); err != nil {
		return nil
	}
	return &Entity{id}
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
