package libtrust

import (
	"encoding/pem"
	"reflect"
	"testing"
)

func TestAddPEMHeadersToKey(t *testing.T) {
	pk := &rsaPublicKey{nil, map[string]interface{}{}}
	blk := &pem.Block{Headers: map[string]string{"hosts": "localhost,127.0.0.1"}}
	addPEMHeadersToKey(blk, pk)

	val := pk.GetExtendedField("hosts")
	hosts, ok := val.([]string)
	if !ok {
		t.Fatalf("hosts type(%v), expected []string", reflect.TypeOf(val))
	}
	expected := []string{"localhost", "127.0.0.1"}
	if !reflect.DeepEqual(hosts, expected) {
		t.Errorf("hosts(%v), expected %v", hosts, expected)
	}
}
