package plugin

import (
	"encoding/json"
	"fmt"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"pgregory.net/rapid"
)

// unmarshalJSON is a copy of the original PluginInterfaceType.UnmarshalJSON
// parser, used to test that the new parser produces the same results for
// well-formed inputs.
func (t *CapabilityID) unmarshalJSON(p []byte) error {
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
		typ := CapabilityID{
			Capability: stringgen.Draw(t, "Capability"),
			Prefix:     stringgen.Draw(t, "Prefix"),
			Version:    stringgen.Draw(t, "Version"),
		}
		b, err := typ.MarshalText()
		if err != nil {
			t.Skipf("unmarshalable value: %v", err)
		}
		t.Logf("InterfaceType(%q)", b)

		var roundtrip CapabilityID
		err = roundtrip.UnmarshalText(b)
		assert.Assert(t, err)
		assert.Assert(t, is.DeepEqual(typ, roundtrip))

		jb, err := json.Marshal(string(b))
		assert.Assert(t, err)
		var oldparser CapabilityID
		err = oldparser.unmarshalJSON(jb)
		assert.Assert(t, err)
		assert.Assert(t, is.DeepEqual(typ, oldparser), "new parser does not match the old parser")
	})
}

func TestCapabilityID_JSONMarshalUnmarshal(t *testing.T) {
	type rt struct {
		Type CapabilityID
	}
	a := rt{
		Type: CapabilityID{
			Capability: "foo",
			Prefix:     "bar",
			Version:    "baz",
		},
	}
	b, err := json.Marshal(a)
	assert.Assert(t, err)
	t.Logf("JSON: %s", b)

	var roundtrip rt
	err = json.Unmarshal(b, &roundtrip)
	assert.Assert(t, err)
	assert.Assert(t, is.DeepEqual(a, roundtrip))
}
