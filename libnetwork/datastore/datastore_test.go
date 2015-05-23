package datastore

import (
	"encoding/json"
	"testing"

	"github.com/docker/libnetwork/config"
	_ "github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/options"
)

var dummyKey = "dummy"

// NewCustomDataStore can be used by other Tests in order to use custom datastore
func NewTestDataStore() DataStore {
	return &datastore{store: NewMockStore()}
}

func TestInvalidDataStore(t *testing.T) {
	config := &config.DatastoreCfg{}
	config.Embedded = false
	config.Client.Provider = "invalid"
	config.Client.Address = "localhost:8500"
	_, err := NewDataStore(config)
	if err == nil {
		t.Fatal("Invalid Datastore connection configuration must result in a failure")
	}
}

func TestKVObjectFlatKey(t *testing.T) {
	store := NewTestDataStore()
	expected := dummyKVObject("1000", true)
	err := store.PutObject(expected)
	if err != nil {
		t.Fatal(err)
	}
	keychain := []string{dummyKey, "1000"}
	data, _, err := store.KVStore().Get(Key(keychain...))
	if err != nil {
		t.Fatal(err)
	}
	var n dummyObject
	json.Unmarshal(data, &n)
	if n.Name != expected.Name {
		t.Fatalf("Dummy object doesnt match the expected object")
	}
}

func TestAtomicKVObjectFlatKey(t *testing.T) {
	store := NewTestDataStore()
	expected := dummyKVObject("1111", true)
	err := store.PutObjectAtomic(expected)
	if err != nil {
		t.Fatal(err)
	}

	// PutObjectAtomic automatically sets the Index again. Hence the following must pass.

	err = store.PutObjectAtomic(expected)
	if err != nil {
		t.Fatal("Atomic update with an older Index must fail")
	}

	// Get the latest index and try PutObjectAtomic again for the same Key
	// This must succeed as well
	data, index, err := store.KVStore().Get(Key(expected.Key()...))
	if err != nil {
		t.Fatal(err)
	}
	n := dummyObject{}
	json.Unmarshal(data, &n)
	n.ID = "1111"
	n.DBIndex = index
	n.ReturnValue = true
	err = store.PutObjectAtomic(&n)
	if err != nil {
		t.Fatal(err)
	}
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
	ReturnValue bool
}

func (n *dummyObject) Key() []string {
	return []string{dummyKey, n.ID}
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

func (n *dummyObject) Index() uint64 {
	return n.DBIndex
}

func (n *dummyObject) SetIndex(index uint64) {
	n.DBIndex = index
}

func (n *dummyObject) MarshalJSON() ([]byte, error) {
	netMap := make(map[string]interface{})
	netMap["name"] = n.Name
	netMap["networkType"] = n.NetworkType
	netMap["enableIPv6"] = n.EnableIPv6
	netMap["generic"] = n.Generic
	return json.Marshal(netMap)
}

func (n *dummyObject) UnmarshalJSON(b []byte) (err error) {
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
	Name    string            `kv:"leaf"`
	Field1  int               `kv:"leaf"`
	Dict    map[string]string `kv:"iterative"`
	DBIndex uint64
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

func (r *recStruct) Index() uint64 {
	return r.DBIndex
}

func (r *recStruct) SetIndex(index uint64) {
	r.DBIndex = index
}

func dummyKVObject(id string, retValue bool) *dummyObject {
	cDict := make(map[string]string)
	cDict["foo"] = "bar"
	cDict["hello"] = "world"
	n := dummyObject{
		Name:        "testNw",
		NetworkType: "bridge",
		EnableIPv6:  true,
		Rec:         &recStruct{"gen", 5, cDict, 0},
		ID:          id,
		DBIndex:     0,
		ReturnValue: retValue}
	generic := make(map[string]interface{})
	generic["label1"] = &recStruct{"value1", 1, cDict, 0}
	generic["label2"] = "subnet=10.1.1.0/16"
	n.Generic = generic
	return &n
}
