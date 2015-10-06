package netlabel

import (
	"testing"

	_ "github.com/docker/libnetwork/testutils"
)

func TestKeyValue(t *testing.T) {
	input := []struct {
		label string
		key   string
		value string
		good  bool
	}{
		{"name=joe", "name", "joe", true},
		{"age=24", "age", "24", true},
		{"address:1234 First st.", "", "", false},
		{"girlfriend=", "girlfriend", "", true},
		{"nickname=o=u=8", "nickname", "o=u=8", true},
		{"", "", "", false},
	}

	for _, i := range input {
		k, v, err := KeyValue(i.label)
		if k != i.key || v != i.value || i.good != (err == nil) {
			t.Fatalf("unexpected: %s, %s, %v", k, v, err)
		}
	}
}

func TestToMap(t *testing.T) {
	input := []struct {
		label string
		key   string
		value string
		good  bool
	}{
		{"name=joe", "name", "joe", true},
		{"age=24", "age", "24", true},
		{"address:1234 First st.", "", "", false},
		{"girlfriend=", "girlfriend", "", true},
		{"nickname=o=u=8", "nickname", "o=u=8", true},
		{"", "", "", false},
	}

	lista := make([]string, len(input))
	for ind, i := range input {
		lista[ind] = i.label
	}

	mappa := ToMap(lista)

	if len(mappa) != len(lista)-2 {
		t.Fatalf("Incorrect map length. Expected %d. Got %d", len(lista), len(mappa))
	}

	for _, i := range input {
		if i.good {
			if v, ok := mappa[i.key]; !ok || v != i.value {
				t.Fatalf("Cannot find key or value for key: %s", i.key)
			}
		} else {
			if _, ok := mappa[i.key]; ok {
				t.Fatalf("Found invalid key in map: %s", i.key)
			}
		}
	}
}
