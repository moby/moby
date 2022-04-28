package manager

import (
	"reflect"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/manager/state/store"
)

// IsStateDirty returns true if any objects have been added to raft which make
// the state "dirty". Currently, the existence of any object other than the
// default cluster or the local node implies a dirty state.
func (m *Manager) IsStateDirty() (bool, error) {
	var (
		storeSnapshot *api.StoreSnapshot
		err           error
	)
	m.raftNode.MemoryStore().View(func(readTx store.ReadTx) {
		storeSnapshot, err = m.raftNode.MemoryStore().Save(readTx)
	})

	if err != nil {
		return false, err
	}

	// Check Nodes and Clusters fields.
	nodeID := m.config.SecurityConfig.ClientTLSCreds.NodeID()
	if len(storeSnapshot.Nodes) > 1 || (len(storeSnapshot.Nodes) == 1 && storeSnapshot.Nodes[0].ID != nodeID) {
		return true, nil
	}

	clusterID := m.config.SecurityConfig.ClientTLSCreds.Organization()
	if len(storeSnapshot.Clusters) > 1 || (len(storeSnapshot.Clusters) == 1 && storeSnapshot.Clusters[0].ID != clusterID) {
		return true, nil
	}

	// Use reflection to check that other fields don't have values. This
	// lets us implement a whitelist-type approach, where we don't need to
	// remember to add individual types here.

	val := reflect.ValueOf(*storeSnapshot)
	numFields := val.NumField()

	for i := 0; i != numFields; i++ {
		field := val.Field(i)
		structField := val.Type().Field(i)
		if structField.Type.Kind() != reflect.Slice {
			panic("unexpected field type in StoreSnapshot")
		}
		if structField.Name != "Nodes" && structField.Name != "Clusters" && structField.Name != "Networks" && field.Len() != 0 {
			// One of the other data types has an entry
			return true, nil
		}
	}

	return false, nil
}
