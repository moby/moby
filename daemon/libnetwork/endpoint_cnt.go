package libnetwork

import (
	"encoding/json"
	"sync"

	"github.com/moby/moby/v2/daemon/libnetwork/datastore"
)

// endpointCnt was used to refcount network-endpoint relationships. It's
// unused since v28.1, and kept around only to ensure that users can properly
// downgrade.
//
// TODO(aker): remove this struct in v30.
type endpointCnt struct {
	n        *Network
	Count    uint64
	dbIndex  uint64
	dbExists bool
	sync.Mutex
}

const epCntKeyPrefix = "endpoint_count"

func (ec *endpointCnt) Key() []string {
	ec.Lock()
	defer ec.Unlock()

	return []string{epCntKeyPrefix, ec.n.id}
}

func (ec *endpointCnt) KeyPrefix() []string {
	ec.Lock()
	defer ec.Unlock()

	return []string{epCntKeyPrefix, ec.n.id}
}

func (ec *endpointCnt) Value() []byte {
	ec.Lock()
	defer ec.Unlock()

	b, err := json.Marshal(ec)
	if err != nil {
		return nil
	}
	return b
}

func (ec *endpointCnt) SetValue(value []byte) error {
	ec.Lock()
	defer ec.Unlock()

	return json.Unmarshal(value, &ec)
}

func (ec *endpointCnt) Index() uint64 {
	ec.Lock()
	defer ec.Unlock()
	return ec.dbIndex
}

func (ec *endpointCnt) SetIndex(index uint64) {
	ec.Lock()
	ec.dbIndex = index
	ec.dbExists = true
	ec.Unlock()
}

func (ec *endpointCnt) Exists() bool {
	ec.Lock()
	defer ec.Unlock()
	return ec.dbExists
}

func (ec *endpointCnt) Skip() bool {
	ec.Lock()
	defer ec.Unlock()
	return !ec.n.persist
}

func (ec *endpointCnt) New() datastore.KVObject {
	ec.Lock()
	defer ec.Unlock()

	return &endpointCnt{
		n: ec.n,
	}
}

func (ec *endpointCnt) CopyTo(o datastore.KVObject) error {
	ec.Lock()
	defer ec.Unlock()

	dstEc := o.(*endpointCnt)
	dstEc.n = ec.n
	dstEc.Count = ec.Count
	dstEc.dbExists = ec.dbExists
	dstEc.dbIndex = ec.dbIndex

	return nil
}
