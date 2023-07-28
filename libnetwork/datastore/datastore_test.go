package datastore

import (
	"encoding/json"
	"testing"

	"github.com/docker/docker/libnetwork/options"
	"github.com/docker/docker/libnetwork/scope"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

const dummyKey = "dummy"

// NewTestDataStore can be used by other Tests in order to use custom datastore
func NewTestDataStore() *Store {
	return &Store{scope: scope.Local, store: NewMockStore()}
}

func TestKey(t *testing.T) {
	sKey := Key("hello", "world")
	const expected = "docker/network/v1.0/hello/world/"
	assert.Check(t, is.Equal(sKey, expected))
}

func TestInvalidDataStore(t *testing.T) {
	_, err := New(ScopeCfg{
		Client: ScopeClientCfg{
			Provider: "invalid",
			Address:  "localhost:8500",
		},
	})
	assert.Check(t, is.Error(err, "unsupported KV store"))
}

func TestKVObjectFlatKey(t *testing.T) {
	store := NewTestDataStore()
	expected := dummyKVObject("1000", true)
	err := store.PutObjectAtomic(expected)
	assert.Check(t, err)

	n := dummyObject{ID: "1000"} // GetObject uses KVObject.Key() for cache lookup.
	err = store.GetObject(Key(dummyKey, "1000"), &n)
	assert.Check(t, err)
	assert.Check(t, is.Equal(n.Name, expected.Name))
}

func TestAtomicKVObjectFlatKey(t *testing.T) {
	store := NewTestDataStore()
	expected := dummyKVObject("1111", true)
	assert.Check(t, !expected.Exists())
	err := store.PutObjectAtomic(expected)
	assert.Check(t, err)
	assert.Check(t, expected.Exists())

	// PutObjectAtomic automatically sets the Index again. Hence the following must pass.

	err = store.PutObjectAtomic(expected)
	assert.Check(t, err, "Atomic update should succeed.")

	// Get the latest index and try PutObjectAtomic again for the same Key
	// This must succeed as well
	n := dummyObject{ID: "1111"} // GetObject uses KVObject.Key() for cache lookup.
	err = store.GetObject(Key(expected.Key()...), &n)
	assert.Check(t, err)
	n.ReturnValue = true
	err = store.PutObjectAtomic(&n)
	assert.Check(t, err)

	// Get the Object using GetObject, then set again.
	newObj := dummyObject{ID: "1111"} // GetObject uses KVObject.Key() for cache lookup.
	err = store.GetObject(Key(expected.Key()...), &newObj)
	assert.Check(t, err)
	assert.Check(t, newObj.Exists())
	err = store.PutObjectAtomic(&n)
	assert.Check(t, err)
}

// dummy data used to test the datastore
type dummyObject struct {
	Name        string                `kv:"leaf"`
	NetworkType string                `kv:"leaf"`
	EnableIPv6  bool                  `kv:"leaf"`
	Rec         *recStruct            `kv:"recursive"`
	Dict        map[string]*recStruct `kv:"iterative"`
	Generic     options.Generic       `kv:"iterative"`
	ID          string
	DBIndex     uint64
	DBExists    bool
	SkipSave    bool
	ReturnValue bool
}

func (n *dummyObject) Key() []string {
	return []string{dummyKey, n.ID}
}

func (n *dummyObject) KeyPrefix() []string {
	return []string{dummyKey}
}

func (n *dummyObject) Value() []byte {
	if !n.ReturnValue {
		return nil
	}

	b, err := json.Marshal(n)
	if err != nil {
		return nil
	}
	return b
}

func (n *dummyObject) SetValue(value []byte) error {
	return json.Unmarshal(value, n)
}

func (n *dummyObject) Index() uint64 {
	return n.DBIndex
}

func (n *dummyObject) SetIndex(index uint64) {
	n.DBIndex = index
	n.DBExists = true
}

func (n *dummyObject) Exists() bool {
	return n.DBExists
}

func (n *dummyObject) Skip() bool {
	return n.SkipSave
}

func (n *dummyObject) DataScope() string {
	return scope.Local
}

func (n *dummyObject) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"name":        n.Name,
		"networkType": n.NetworkType,
		"enableIPv6":  n.EnableIPv6,
		"generic":     n.Generic,
	})
}

func (n *dummyObject) UnmarshalJSON(b []byte) error {
	var netMap map[string]interface{}
	if err := json.Unmarshal(b, &netMap); err != nil {
		return err
	}
	n.Name = netMap["name"].(string)
	n.NetworkType = netMap["networkType"].(string)
	n.EnableIPv6 = netMap["enableIPv6"].(bool)
	n.Generic = netMap["generic"].(map[string]interface{})
	return nil
}

// dummy structure to test "recursive" cases
type recStruct struct {
	Name     string            `kv:"leaf"`
	Field1   int               `kv:"leaf"`
	Dict     map[string]string `kv:"iterative"`
	DBIndex  uint64
	DBExists bool
	SkipSave bool
}

func (r *recStruct) Key() []string {
	return []string{"recStruct"}
}

func (r *recStruct) Value() []byte {
	b, err := json.Marshal(r)
	if err != nil {
		return nil
	}
	return b
}

func (r *recStruct) SetValue(value []byte) error {
	return json.Unmarshal(value, r)
}

func (r *recStruct) Index() uint64 {
	return r.DBIndex
}

func (r *recStruct) SetIndex(index uint64) {
	r.DBIndex = index
	r.DBExists = true
}

func (r *recStruct) Exists() bool {
	return r.DBExists
}

func (r *recStruct) Skip() bool {
	return r.SkipSave
}

func dummyKVObject(id string, retValue bool) *dummyObject {
	cDict := map[string]string{
		"foo":   "bar",
		"hello": "world",
	}
	return &dummyObject{
		Name:        "testNw",
		NetworkType: "bridge",
		EnableIPv6:  true,
		Rec:         &recStruct{Name: "gen", Field1: 5, Dict: cDict},
		ID:          id,
		DBIndex:     0,
		ReturnValue: retValue,
		DBExists:    false,
		SkipSave:    false,
		Generic: map[string]interface{}{
			"label1": &recStruct{Name: "value1", Field1: 1, Dict: cDict},
			"label2": "subnet=10.1.1.0/16",
		},
	}
}
