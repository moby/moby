package container

import (
	"fmt"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/registrar"
	"github.com/docker/go-connections/nat"
	"github.com/hashicorp/go-memdb"
)

const (
	memdbTable   = "containers"
	memdbIDIndex = "id"
)

// Snapshot is a read only view for Containers. It holds all information necessary to serve container queries in a
// versioned ACID in-memory store.
type Snapshot struct {
	types.Container

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
	ExposedPorts nat.PortSet
	PortBindings nat.PortSet
	Health       string
	HostConfig   struct {
		Isolation string
	}
}

// ViewDB provides an in-memory transactional (ACID) container Store
type ViewDB interface {
	Snapshot(nameIndex *registrar.Registrar) View
	Save(*Container) error
	Delete(*Container) error
}

// View can be used by readers to avoid locking
type View interface {
	All() ([]Snapshot, error)
	Get(id string) (*Snapshot, error)
}

var schema = &memdb.DBSchema{
	Tables: map[string]*memdb.TableSchema{
		memdbTable: {
			Name: memdbTable,
			Indexes: map[string]*memdb.IndexSchema{
				memdbIDIndex: {
					Name:    memdbIDIndex,
					Unique:  true,
					Indexer: &containerByIDIndexer{},
				},
			},
		},
	},
}

type memDB struct {
	store *memdb.MemDB
}

// NewViewDB provides the default implementation, with the default schema
func NewViewDB() (ViewDB, error) {
	store, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, err
	}
	return &memDB{store: store}, nil
}

// Snapshot provides a consistent read-only View of the database
func (db *memDB) Snapshot(index *registrar.Registrar) View {
	return &memdbView{
		txn:       db.store.Txn(false),
		nameIndex: index.GetAll(),
	}
}

// Save atomically updates the in-memory store state for a Container.
// Only read only (deep) copies of containers may be passed in.
func (db *memDB) Save(c *Container) error {
	txn := db.store.Txn(true)
	defer txn.Commit()
	return txn.Insert(memdbTable, c)
}

// Delete removes an item by ID
func (db *memDB) Delete(c *Container) error {
	txn := db.store.Txn(true)
	defer txn.Commit()
	return txn.Delete(memdbTable, NewBaseContainer(c.ID, c.Root))
}

type memdbView struct {
	txn       *memdb.Txn
	nameIndex map[string][]string
}

// All returns a all items in this snapshot. Returned objects must never be modified.
func (v *memdbView) All() ([]Snapshot, error) {
	var all []Snapshot
	iter, err := v.txn.Get(memdbTable, memdbIDIndex)
	if err != nil {
		return nil, err
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
func (v *memdbView) Get(id string) (*Snapshot, error) {
	s, err := v.txn.First(memdbTable, memdbIDIndex, id)
	if err != nil {
		return nil, err
	}
	return v.transform(s.(*Container)), nil
}

// transform maps a (deep) copied Container object to what queries need.
// A lock on the Container is not held because these are immutable deep copies.
func (v *memdbView) transform(container *Container) *Snapshot {
	snapshot := &Snapshot{
		Container: types.Container{
			ID:      container.ID,
			Names:   v.nameIndex[container.ID],
			ImageID: container.ImageID.String(),
			Ports:   []types.Port{},
			Mounts:  container.GetMountPoints(),
			State:   container.State.StateString(),
			Status:  container.State.String(),
			Created: container.Created.Unix(),
		},
		CreatedAt:    container.Created,
		StartedAt:    container.StartedAt,
		Name:         container.Name,
		Pid:          container.Pid,
		Managed:      container.Managed,
		ExposedPorts: make(nat.PortSet),
		PortBindings: make(nat.PortSet),
		Health:       container.HealthString(),
		Running:      container.Running,
		Paused:       container.Paused,
		ExitCode:     container.ExitCode(),
	}

	if snapshot.Names == nil {
		// Dead containers will often have no name, so make sure the response isn't null
		snapshot.Names = []string{}
	}

	if container.HostConfig != nil {
		snapshot.Container.HostConfig.NetworkMode = string(container.HostConfig.NetworkMode)
		snapshot.HostConfig.Isolation = string(container.HostConfig.Isolation)
		for binding := range container.HostConfig.PortBindings {
			snapshot.PortBindings[binding] = struct{}{}
		}
	}

	if container.Config != nil {
		snapshot.Image = container.Config.Image
		snapshot.Labels = container.Config.Labels
		for exposed := range container.Config.ExposedPorts {
			snapshot.ExposedPorts[exposed] = struct{}{}
		}
	}

	if len(container.Args) > 0 {
		args := []string{}
		for _, arg := range container.Args {
			if strings.Contains(arg, " ") {
				args = append(args, fmt.Sprintf("'%s'", arg))
			} else {
				args = append(args, arg)
			}
		}
		argsAsString := strings.Join(args, " ")
		snapshot.Command = fmt.Sprintf("%s %s", container.Path, argsAsString)
	} else {
		snapshot.Command = container.Path
	}

	snapshot.Ports = []types.Port{}
	networks := make(map[string]*network.EndpointSettings)
	if container.NetworkSettings != nil {
		for name, netw := range container.NetworkSettings.Networks {
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
			}
			if netw.IPAMConfig != nil {
				networks[name].IPAMConfig = &network.EndpointIPAMConfig{
					IPv4Address: netw.IPAMConfig.IPv4Address,
					IPv6Address: netw.IPAMConfig.IPv6Address,
				}
			}
		}
		for port, bindings := range container.NetworkSettings.Ports {
			p, err := nat.ParsePort(port.Port())
			if err != nil {
				logrus.Warnf("invalid port map %+v", err)
				continue
			}
			if len(bindings) == 0 {
				snapshot.Ports = append(snapshot.Ports, types.Port{
					PrivatePort: uint16(p),
					Type:        port.Proto(),
				})
				continue
			}
			for _, binding := range bindings {
				h, err := nat.ParsePort(binding.HostPort)
				if err != nil {
					logrus.Warnf("invalid host port map %+v", err)
					continue
				}
				snapshot.Ports = append(snapshot.Ports, types.Port{
					PrivatePort: uint16(p),
					PublicPort:  uint16(h),
					Type:        port.Proto(),
					IP:          binding.HostIP,
				})
			}
		}
	}
	snapshot.NetworkSettings = &types.SummaryNetworkSettings{Networks: networks}

	return snapshot
}

// containerByIDIndexer is used to extract the ID field from Container types.
// memdb.StringFieldIndex can not be used since ID is a field from an embedded struct.
type containerByIDIndexer struct{}

// FromObject implements the memdb.SingleIndexer interface for Container objects
func (e *containerByIDIndexer) FromObject(obj interface{}) (bool, []byte, error) {
	c, ok := obj.(*Container)
	if !ok {
		return false, nil, fmt.Errorf("%T is not a Container", obj)
	}
	// Add the null character as a terminator
	v := c.ID + "\x00"
	return true, []byte(v), nil
}

// FromArgs implements the memdb.Indexer interface
func (e *containerByIDIndexer) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	// Add the null character as a terminator
	arg += "\x00"
	return []byte(arg), nil
}
