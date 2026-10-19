package plugin_test

import (
	"testing"

	"github.com/moby/moby/api/types/plugin"
)

func FuzzCapabilityID(f *testing.F) {
	f.Add("docker", "networkdriver", "1.0")
	f.Add("foo.bar", "baz", "1.0")
	f.Add("", "bar", "1.0")

	f.Fuzz(func(t *testing.T, prefix, capability, version string) {
		original := plugin.CapabilityID{
			Prefix:     prefix,
			Capability: capability,
			Version:    version,
		}

		b, err := original.MarshalText()
		if err != nil {
			return // Invalid combination.
		}

		var got plugin.CapabilityID
		if err := got.UnmarshalText(b); err != nil {
			t.Fatal(err)
		}
		if got != original {
			t.Fatalf("roundtrip = %+v, want %+v", got, original)
		}
	})
}
