package plugin_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/moby/moby/api/types/plugin"
	"pgregory.net/rapid"
)

type pluginCapabilityID plugin.CapabilityID

// unmarshalJSON is a copy of the original PluginInterfaceType.UnmarshalJSON
// parser, used to test that the new parser produces the same results for
// well-formed inputs.
func (t *pluginCapabilityID) unmarshalJSON(p []byte) error {
	versionIndex := len(p)
	prefixIndex := 0
	if len(p) < 2 || p[0] != '"' || p[len(p)-1] != '"' {
		return fmt.Errorf("%q is not a plugin interface type", p)
	}
	p = p[1 : len(p)-1]
loop:
	for i, b := range p {
		switch b {
		case '.':
			prefixIndex = i
		case '/':
			versionIndex = i
			break loop
		}
	}
	t.Prefix = string(p[:prefixIndex])
	t.Capability = string(p[prefixIndex+1 : versionIndex])
	if versionIndex < len(p) {
		t.Version = string(p[versionIndex+1:])
	}
	return nil
}

func TestCapabilityID_MarshalUnmarshal(t *testing.T) {
	stringgen := rapid.StringMatching(`[a-z0-9-./]*`)
	rapid.Check(t, func(t *rapid.T) {
		typ := plugin.CapabilityID{
			Capability: stringgen.Draw(t, "Capability"),
			Prefix:     stringgen.Draw(t, "Prefix"),
			Version:    stringgen.Draw(t, "Version"),
		}
		b, err := typ.MarshalText()
		if err != nil {
			t.Skipf("unmarshalable value: %v", err)
		}
		t.Logf("InterfaceType(%q)", b)

		var roundtrip plugin.CapabilityID
		if err := roundtrip.UnmarshalText(b); err != nil {
			t.Fatal(err)
		}
		if typ != roundtrip {
			t.Errorf("roundtrip = %+v, want %+v", roundtrip, typ)
		}

		jb, err := json.Marshal(string(b))
		if err != nil {
			t.Fatal(err)
		}

		var oldparser pluginCapabilityID
		if err := oldparser.unmarshalJSON(jb); err != nil {
			t.Fatal(err)
		}
		if typ != plugin.CapabilityID(oldparser) {
			t.Errorf("new parser = %+v, old parser = %+v", typ, oldparser)
		}
	})
}

func TestCapabilityID_JSONMarshalUnmarshal(t *testing.T) {
	type rt struct {
		Type plugin.CapabilityID
	}
	a := rt{
		Type: plugin.CapabilityID{
			Capability: "foo",
			Prefix:     "bar",
			Version:    "baz",
		},
	}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("JSON: %s", b)

	var roundtrip rt
	if err := json.Unmarshal(b, &roundtrip); err != nil {
		t.Fatal(err)
	}
	if a != roundtrip {
		t.Errorf("roundtrip = %+v, want %+v", roundtrip, a)
	}
}
