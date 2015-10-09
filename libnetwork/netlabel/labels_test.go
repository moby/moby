package netlabel

import (
	"testing"

	_ "github.com/docker/libnetwork/testutils"
)

var input = []struct {
	label string
	key   string
	value string
}{
	{"com.directory.person.name=joe", "com.directory.person.name", "joe"},
	{"com.directory.person.age=24", "com.directory.person.age", "24"},
	{"com.directory.person.address=1234 First st.", "com.directory.person.address", "1234 First st."},
	{"com.directory.person.friends=", "com.directory.person.friends", ""},
	{"com.directory.person.nickname=o=u=8", "com.directory.person.nickname", "o=u=8"},
	{"", "", ""},
	{"com.directory.person.student", "com.directory.person.student", ""},
}

func TestKeyValue(t *testing.T) {
	for _, i := range input {
		k, v := KeyValue(i.label)
		if k != i.key || v != i.value {
			t.Fatalf("unexpected: %s, %s", k, v)
		}
	}
}

func TestToMap(t *testing.T) {
	lista := make([]string, len(input))
	for ind, i := range input {
		lista[ind] = i.label
	}

	mappa := ToMap(lista)

	if len(mappa) != len(lista) {
		t.Fatalf("Incorrect map length. Expected %d. Got %d", len(lista), len(mappa))
	}

	for _, i := range input {
		if v, ok := mappa[i.key]; !ok || v != i.value {
			t.Fatalf("Cannot find key or value for key: %s", i.key)
		}
	}
}

func TestFromMap(t *testing.T) {
	var m map[string]string
	lbls := FromMap(m)
	if len(lbls) != 0 {
		t.Fatalf("unexpected lbls length")
	}

	m = make(map[string]string, 3)
	m["peso"] = "85"
	m["statura"] = "170"
	m["maschio"] = ""

	lbls = FromMap(m)
	if len(lbls) != 3 {
		t.Fatalf("unexpected lbls length")
	}

	for _, l := range lbls {
		switch l {
		case "peso=85":
		case "statura=170":
		case "maschio":
		default:
			t.Fatalf("unexpected label: %s", l)
		}
	}
}
