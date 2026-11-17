package plugin_test

import (
	"encoding/json"
	"testing"

	"github.com/moby/moby/api/types/plugin"
)

func TestCapabilityID_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		id   plugin.CapabilityID
		want string
	}{
		{
			name: "standard",
			id:   plugin.CapabilityID{Prefix: "docker", Capability: "networkdriver", Version: "1.0"},
			want: "docker.networkdriver/1.0",
		},
		{
			name: "dotted prefix",
			id:   plugin.CapabilityID{Prefix: "com.example", Capability: "driver", Version: "1.0"},
			want: "com.example.driver/1.0",
		},
		{
			name: "empty prefix",
			id:   plugin.CapabilityID{Capability: "driver", Version: "1.0"},
			want: ".driver/1.0",
		},
		{
			name: "empty version",
			id:   plugin.CapabilityID{Prefix: "docker", Capability: "driver"},
			want: "docker.driver/",
		},
		{
			name: "zero value",
			id:   plugin.CapabilityID{},
			want: "./",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := tc.id.MarshalText()
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != tc.want {
				t.Errorf("MarshalText() = %q, want %q", b, tc.want)
			}

			var got plugin.CapabilityID
			if err := got.UnmarshalText(b); err != nil {
				t.Fatal(err)
			}
			if got != tc.id {
				t.Errorf("roundtrip = %+v, want %+v", got, tc.id)
			}
		})
	}
}

// TestCapabilityID_JSONMarshalUnmarshal verifies that encoding/json uses
// the text marshaling methods and preserves the JSON representation.
func TestCapabilityID_JSONMarshalUnmarshal(t *testing.T) {
	id := plugin.CapabilityID{Prefix: "docker", Capability: "networkdriver", Version: "1.0"}

	data, err := json.Marshal(id)
	if err != nil {
		t.Fatal(err)
	}
	if want := `"docker.networkdriver/1.0"`; string(data) != want {
		t.Errorf("json.Marshal() = %s, want %s", data, want)
	}

	var got plugin.CapabilityID
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Errorf("JSON roundtrip = %+v, want %+v", got, id)
	}
}

func TestCapabilityID_JSONUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		data string
		want plugin.CapabilityID
	}{
		{
			name: "standard",
			data: `"docker.networkdriver/1.0"`,
			want: plugin.CapabilityID{
				Prefix:     "docker",
				Capability: "networkdriver",
				Version:    "1.0",
			},
		},
		{
			name: "dotted prefix",
			data: `"com.example.driver/1.0"`,
			want: plugin.CapabilityID{
				Prefix:     "com.example",
				Capability: "driver",
				Version:    "1.0",
			},
		},
		{
			name: "no version",
			data: `"docker.driver"`,
			want: plugin.CapabilityID{
				Prefix:     "docker",
				Capability: "driver",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got plugin.CapabilityID
			if err := json.Unmarshal([]byte(tc.data), &got); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("UnmarshalJSON(%s) = %+v, want %+v", tc.data, got, tc.want)
			}
		})
	}
}
