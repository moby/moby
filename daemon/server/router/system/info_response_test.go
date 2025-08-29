package system

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/api/types/system"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestLegacyFields(t *testing.T) {
	infoResp := &infoResponse{
		Info: &system.Info{
			Containers: 10,
		},
		extraFields: map[string]any{
			"LegacyFoo": false,
			"LegacyBar": true,
		},
	}

	data, err := json.MarshalIndent(infoResp, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if expected := `"LegacyFoo": false`; !strings.Contains(string(data), expected) {
		t.Errorf("legacy fields should contain %s: %s", expected, string(data))
	}
	if expected := `"LegacyBar": true`; !strings.Contains(string(data), expected) {
		t.Errorf("legacy fields should contain %s: %s", expected, string(data))
	}
}

// TestMarshalRegistryConfigLegacyFields verifies extra fields in the registry config
// field in the info response are serialized if they are not empty.
// This is used for backwards compatibility for API versions < 1.47.
func TestMarshalRegistryConfigLegacyFields(t *testing.T) {
	expected := []string{"AllowNondistributableArtifactsCIDRs", "AllowNondistributableArtifactsHostnames"}

	tests := []struct {
		name   string
		info   *infoResponse
		assert func(t *testing.T, data []byte, err error)
	}{
		{
			name: "without legacy fields",
			info: &infoResponse{
				Info: &system.Info{},
			},
			assert: func(t *testing.T, data []byte, err error) {
				assert.NilError(t, err)

				var resp map[string]any
				err = json.Unmarshal(data, &resp)
				assert.NilError(t, err)

				rc, ok := resp["RegistryConfig"]
				assert.Check(t, ok)

				for _, v := range expected {
					assert.Check(t, !is.Contains(rc, v)().Success())
				}
			},
		},
		{
			name: "with legacy fields",
			info: &infoResponse{
				Info: &system.Info{
					RegistryConfig: &registry.ServiceConfig{
						ExtraFields: map[string]any{
							"AllowNondistributableArtifactsCIDRs":     json.RawMessage(nil),
							"AllowNondistributableArtifactsHostnames": json.RawMessage(nil),
						},
					},
				},
			},
			assert: func(t *testing.T, data []byte, err error) {
				assert.NilError(t, err)

				var resp map[string]any
				err = json.Unmarshal(data, &resp)
				assert.NilError(t, err)

				rc, ok := resp["RegistryConfig"]
				assert.Check(t, ok)

				for _, v := range expected {
					assert.Check(t, is.Contains(rc, v))
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.MarshalIndent(tc.info, "", "  ")
			tc.assert(t, data, err)
		})
	}
}
