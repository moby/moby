package container

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/containerd/log"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/v2/errdefs"
)

const (
	memdbContainersTable  = "containers"
	memdbNamesTable       = "names"
	memdbIDIndex          = "id"
	memdbIDIndexPrefix    = "id_prefix"
	memdbContainerIDIndex = "containerid"
)

// Snapshot is a read only view for Containers. It holds all information necessary to serve container queries in a
// versioned ACID in-memory store.
type Snapshot struct {
	container.Summary

	// additional info queries need to filter on
	// preserve nanosec resolution for queries
	CreatedAt    time.Time
	StartedAt    time.Time
	Name         string
	Pid          int
	ExitCode     int
	Running      bool
	Paused       bool
	Managed      bool
	ExposedPorts network.PortSet
	PortBindings network.PortSet
	Health       container.HealthStatus
	HostConfig   struct {
		Isolation string
	}
}

// nameAssociation associates a container id with a name.
type nameAssociation struct {
	// name is the name to associate. Note that name is the primary key
	// ("id" in memdb).
	name        string
	containerID string
}

var schema = &memdb.DBSchema{
	Tables: map[string]*memdb.TableSchema{
		memdbContainersTable: {
			Name: memdbContainersTable,
			Indexes: map[string]*memdb.IndexSchema{
				memdbIDIndex: {
					Name:    memdbIDIndex,
					Unique:  true,
					Indexer: &containerByIDIndexer{},
				},
			},
		},
		memdbNamesTable: {
			Name: memdbNamesTable,
			Indexes: map[string]*memdb.IndexSchema{
				// Used for names, because "id" is the primary key in memdb.
				memdbIDIndex: {
					Name:    memdbIDIndex,
					Unique:  true,
					Indexer: &namesByNameIndexer{},
				},
				memdbContainerIDIndex: {
					Name:    memdbContainerIDIndex,
					Indexer: &namesByContainerIDIndexer{},
				},
			},
		},
	},
}

// ViewDB provides an in-memory transactional (ACID) container store.
type ViewDB struct {
	store *memdb.MemDB
}

// NewViewDB provides the default implementation, with the default schema
func NewViewDB() (*ViewDB, error) {
	store, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, errdefs.System(err)
	}
	return &ViewDB{store: store}, nil
}

// GetByPrefix returns a container with the given ID prefix. It returns an
// error if an empty prefix was given or if multiple containers match the prefix.
// It returns an [errdefs.NotFound] if the given s yielded no results.
func (db *ViewDB) GetByPrefix(s string) (string, error) {
	if s == "" {
		return "", errdefs.InvalidParameter(errors.New("prefix can't be empty"))
	}
	iter, err := db.store.Txn(false).Get(memdbContainersTable, memdbIDIndexPrefix, s)
	if err != nil {
		return "", errdefs.System(err)
	}

	var id string
	for {
		item := iter.Next()
		if item == nil {
			break
		}
		if id != "" {
			return "", errdefs.InvalidParameter(errors.New("multiple IDs found with provided prefix: " + s))
		}
		id = item.(*Container).ID
	}

	if id != "" {
		return id, nil
	}

	return "", errdefs.NotFound(errors.New("No such container: " + s))
}

// Snapshot provides a consistent read-only view of the database.
func (db *ViewDB) Snapshot() *View {
	return &View{
		txn: db.store.Txn(false),
	}
}

func (db *ViewDB) withTxn(cb func(*memdb.Txn) error) error {
	txn := db.store.Txn(true)
	err := cb(txn)
	if err != nil {
		txn.Abort()
		return err
	}
	txn.Commit()
	return nil
}

// Save atomically updates the in-memory store state for a Container.
// Only read-only (deep) copies of containers may be passed in.
func (db *ViewDB) Save(c *Container) error {
	return db.withTxn(func(txn *memdb.Txn) error {
		return txn.Insert(memdbContainersTable, c)
	})
}

// Delete removes a container by its ID and releases all names associated
// with it. Delete is idempotent, and ignores errors due to the container
// not existing.
func (db *ViewDB) Delete(containerID string) {
	_ = db.withTxn(func(txn *memdb.Txn) error {
		view := &View{txn: txn}

		// Clean up all names associated with the container; ignore
		// errors, as names may not be found and we need to clean up
		// the container itself after this.
		for _, name := range view.getNames(containerID) {
			_ = txn.Delete(memdbNamesTable, nameAssociation{name: name})
		}

		// Ignore error - the container may not actually exist.
		_ = txn.Delete(memdbContainersTable, containerIDKey(containerID))
		return nil
	})
}

// ReserveName registers a container ID to a name. ReserveName is idempotent,
// but returns an [errdefs.Conflict] when attempting to reserve a container ID
// to a name that already is reserved.
func (db *ViewDB) ReserveName(name, containerID string) error {
	return db.withTxn(func(txn *memdb.Txn) error {
		s, err := txn.First(memdbNamesTable, memdbIDIndex, name)
		if err != nil {
			return errdefs.System(err)
		}
		if s != nil {
			if s.(nameAssociation).containerID != containerID {
				return errdefs.Conflict(errors.New("name is reserved"))
			}
			return nil
		}
		return txn.Insert(memdbNamesTable, nameAssociation{name: name, containerID: containerID})
	})
}

// ReleaseName releases the reserved name
// Once released, a name can be reserved again
func (db *ViewDB) ReleaseName(name string) error {
	return db.withTxn(func(txn *memdb.Txn) error {
		return txn.Delete(memdbNamesTable, nameAssociation{name: name})
	})
}

// View provides a consistent read-only view of the database.
type View struct {
	txn *memdb.Txn
}

// All returns a all items in this snapshot. Returned objects must never be modified.
func (v *View) All() ([]Snapshot, error) {
	var all []Snapshot
	iter, err := v.txn.Get(memdbContainersTable, memdbIDIndex)
	if err != nil {
		return nil, errdefs.System(err)
	}
	for {
		item := iter.Next()
		if item == nil {
			break
		}
		snapshot := v.transform(item.(*Container))
		all = append(all, *snapshot)
	}
	return all, nil
}

// Get returns an item by id. Returned objects must never be modified.
// It returns an [errdefs.NotFound] if the given id was not found.
func (v *View) Get(id string) (*Snapshot, error) {
	s, err := v.txn.First(memdbContainersTable, memdbIDIndex, id)
	if err != nil {
		return nil, errdefs.System(err)
	}
	if s == nil {
		return nil, errdefs.NotFound(errors.New("No such container: " + id))
	}
	return v.transform(s.(*Container)), nil
}

// getNames lists all the reserved names for the given container ID.
func (v *View) getNames(containerID string) []string {
	iter, err := v.txn.Get(memdbNamesTable, memdbContainerIDIndex, containerID)
	if err != nil {
		return nil
	}

	var names []string
	for {
		item := iter.Next()
		if item == nil {
			break
		}
		names = append(names, item.(nameAssociation).name)
	}

	return names
}

// GetID returns the container ID that the passed in name is reserved to.
// It returns an [errdefs.NotFound] if the given id was not found.
func (v *View) GetID(name string) (string, error) {
	s, err := v.txn.First(memdbNamesTable, memdbIDIndex, name)
	if err != nil {
		return "", errdefs.System(err)
	}
	if s == nil {
		return "", errdefs.NotFound(errors.New("name is not reserved"))
	}
	return s.(nameAssociation).containerID, nil
}

// GetAllNames returns all registered names.
func (v *View) GetAllNames() map[string][]string {
	iter, err := v.txn.Get(memdbNamesTable, memdbContainerIDIndex)
	if err != nil {
		return nil
	}

	out := make(map[string][]string)
	for {
		item := iter.Next()
		if item == nil {
			break
		}
		assoc := item.(nameAssociation)
		out[assoc.containerID] = append(out[assoc.containerID], assoc.name)
	}

	return out
}

// transform maps a (deep) copied Container object to what queries need.
// A lock on the Container is not held because these are immutable deep copies.
func (v *View) transform(ctr *Container) *Snapshot {
	health := container.NoHealthcheck
	failingStreak := 0
	if ctr.State.Health != nil {
		health = ctr.State.Health.Status()
		failingStreak = ctr.State.Health.Health.FailingStreak
	}

	snapshot := &Snapshot{
		Summary: container.Summary{
			ID:      ctr.ID,
			Names:   v.getNames(ctr.ID),
			ImageID: ctr.ImageID.String(),
			Ports:   []container.PortSummary{},
			Mounts:  ctr.GetMountPoints(),
			State:   ctr.State.State(),
			Status:  ctr.State.String(),
			Health: &container.HealthSummary{
				Status:        health,
				FailingStreak: failingStreak,
			},
			Created: ctr.Created.Unix(),
		},
		CreatedAt:    ctr.Created,
		StartedAt:    ctr.State.StartedAt,
		Name:         ctr.Name,
		Pid:          ctr.State.Pid,
		Managed:      ctr.Managed,
		ExposedPorts: make(network.PortSet),
		PortBindings: make(network.PortSet),
		Health:       health,
		Running:      ctr.State.Running,
		Paused:       ctr.State.Paused,
		ExitCode:     ctr.State.ExitCode,
	}

	if snapshot.Names == nil {
		// Dead containers will often have no name, so make sure the response isn't null
		snapshot.Names = []string{}
	}

	if ctr.HostConfig != nil {
		snapshot.Summary.HostConfig.NetworkMode = string(ctr.HostConfig.NetworkMode)
		snapshot.Summary.HostConfig.Annotations = maps.Clone(ctr.HostConfig.Annotations)
		snapshot.HostConfig.Isolation = string(ctr.HostConfig.Isolation)
		for binding := range ctr.HostConfig.PortBindings {
			snapshot.PortBindings[binding] = struct{}{}
		}
	}

	if ctr.Config != nil {
		snapshot.Image = ctr.Config.Image
		snapshot.Labels = ctr.Config.Labels
		for exposed := range ctr.Config.ExposedPorts {
			snapshot.ExposedPorts[exposed] = struct{}{}
		}
	}

	if len(ctr.Args) > 0 {
		var args []string
		for _, arg := range ctr.Args {
			if strings.Contains(arg, " ") {
				args = append(args, fmt.Sprintf("'%s'", arg))
			} else {
				args = append(args, arg)
			}
		}
		argsAsString := strings.Join(args, " ")
		snapshot.Command = fmt.Sprintf("%s %s", ctr.Path, argsAsString)
	} else {
		snapshot.Command = ctr.Path
	}

	snapshot.Ports = []container.PortSummary{}
	networks := make(map[string]*network.EndpointSettings)
	if ctr.NetworkSettings != nil {
		for name, netw := range ctr.NetworkSettings.Networks {
			if netw == nil || netw.EndpointSettings == nil {
				continue
			}
			networks[name] = &network.EndpointSettings{
				EndpointID:          netw.EndpointID,
				Gateway:             netw.Gateway,
				IPAddress:           netw.IPAddress,
				IPPrefixLen:         netw.IPPrefixLen,
				IPv6Gateway:         netw.IPv6Gateway,
				GlobalIPv6Address:   netw.GlobalIPv6Address,
				GlobalIPv6PrefixLen: netw.GlobalIPv6PrefixLen,
				MacAddress:          netw.MacAddress,
				NetworkID:           netw.NetworkID,
				GwPriority:          netw.GwPriority,
			}
			if netw.IPAMConfig != nil {
				networks[name].IPAMConfig = &network.EndpointIPAMConfig{
					IPv4Address: netw.IPAMConfig.IPv4Address,
					IPv6Address: netw.IPAMConfig.IPv6Address,
				}
			}
		}
		for p, bindings := range ctr.NetworkSettings.Ports {
			if len(bindings) == 0 {
				snapshot.Ports = append(snapshot.Ports, container.PortSummary{
					PrivatePort: p.Num(),
					Type:        string(p.Proto()),
				})
				continue
			}
			for _, binding := range bindings {
				// TODO(thaJeztah): if this is always a port/proto (no range), we can simplify this to [network.ParsePort].
				h, err := network.ParsePortRange(binding.HostPort)
				if err != nil {
					log.G(context.TODO()).WithError(err).Warn("invalid host port map")
					continue
				}
				snapshot.Ports = append(snapshot.Ports, container.PortSummary{
					PrivatePort: p.Num(),
					PublicPort:  h.Start(),
					Type:        string(p.Proto()),
					IP:          binding.HostIP,
				})
			}
		}
	}
	snapshot.NetworkSettings = &container.NetworkSettingsSummary{Networks: networks}

	if ctr.ImageManifest != nil {
		imageManifest := *ctr.ImageManifest
		if imageManifest.Platform == nil {
			imageManifest.Platform = &ctr.ImagePlatform
		}
		snapshot.Summary.ImageManifestDescriptor = &imageManifest
	}

	return snapshot
}

// terminator is the null character, used as a terminator.
const terminator = "\x00"

// containerIDKey is used to lookup a container by its ID. It's an alternative
// to passing a [Container] struct for situations where no Container struct
// is available.
type containerIDKey string

// containerByIDIndexer is used to extract the ID field from Container types.
// memdb.StringFieldIndex can not be used since ID is a field from an embedded struct.
type containerByIDIndexer struct{}

// FromObject implements the memdb.SingleIndexer interface for Container objects
func (e *containerByIDIndexer) FromObject(obj any) (bool, []byte, error) {
	switch c := obj.(type) {
	case containerIDKey:
		// Add the null character as a terminator
		return true, []byte(c + terminator), nil
	case *Container:
		// Add the null character as a terminator
		return true, []byte(c.ID + terminator), nil
	default:
		return false, nil, fmt.Errorf("%T is not a Container", obj)
	}
}

// FromArgs implements the memdb.Indexer interface
func (e *containerByIDIndexer) FromArgs(args ...any) ([]byte, error) {
	if len(args) != 1 {
		return nil, errors.New("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	// Add the null character as a terminator
	return []byte(arg + terminator), nil
}

func (e *containerByIDIndexer) PrefixFromArgs(args ...any) ([]byte, error) {
	val, err := e.FromArgs(args...)
	if err != nil {
		return nil, err
	}

	// Strip the null terminator, the rest is a prefix
	return bytes.TrimSuffix(val, []byte(terminator)), nil
}

// namesByNameIndexer is used to index container name associations by name.
type namesByNameIndexer struct{}

func (e *namesByNameIndexer) FromObject(obj any) (bool, []byte, error) {
	n, ok := obj.(nameAssociation)
	if !ok {
		return false, nil, fmt.Errorf(`%T does not have type "nameAssociation"`, obj)
	}

	// Add the null character as a terminator
	return true, []byte(n.name + terminator), nil
}

func (e *namesByNameIndexer) FromArgs(args ...any) ([]byte, error) {
	if len(args) != 1 {
		return nil, errors.New("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	// Add the null character as a terminator
	return []byte(arg + terminator), nil
}

// namesByContainerIDIndexer is used to index container names by container ID.
type namesByContainerIDIndexer struct{}

func (e *namesByContainerIDIndexer) FromObject(obj any) (bool, []byte, error) {
	n, ok := obj.(nameAssociation)
	if !ok {
		return false, nil, fmt.Errorf(`%T does not have type "nameAssociation"`, obj)
	}

	// Add the null character as a terminator
	return true, []byte(n.containerID + terminator), nil
}

func (e *namesByContainerIDIndexer) FromArgs(args ...any) ([]byte, error) {
	if len(args) != 1 {
		return nil, errors.New("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	// Add the null character as a terminator
	return []byte(arg + terminator), nil
}
