package image

import (
	"encoding/json"
	"testing"

	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/moby/moby/api/types/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestInspectResponse(t *testing.T) {
	tests := []struct {
		doc          string
		cfg          *ocispec.ImageConfig
		legacyConfig map[string]any
		expected     string
	}{
		{
			doc:      "empty",
			expected: `null`,
		},
		{
			doc: "no legacy config",
			cfg: &ocispec.ImageConfig{
				Cmd:        []string{"/bin/sh"},
				StopSignal: "SIGQUIT",
			},
			expected: `{"Cmd":["/bin/sh"],"StopSignal":"SIGQUIT"}`,
		},
		{
			doc: "api < v1.50",
			cfg: &ocispec.ImageConfig{
				Cmd:        []string{"/bin/sh"},
				StopSignal: "SIGQUIT",
			},
			legacyConfig: legacyConfigFields["v1.49"],
			expected:     `{"AttachStderr":false,"AttachStdin":false,"AttachStdout":false,"Cmd":["/bin/sh"],"Domainname":"","Entrypoint":null,"Env":null,"Hostname":"","Image":"","Labels":null,"OnBuild":null,"OpenStdin":false,"StdinOnce":false,"StopSignal":"SIGQUIT","Tty":false,"User":"","Volumes":null,"WorkingDir":""}`,
		},
		{
			doc: "api >= v1.50",
			cfg: &ocispec.ImageConfig{
				Cmd:        []string{"/bin/sh"},
				StopSignal: "SIGQUIT",
			},
			legacyConfig: legacyConfigFields["current"],
			expected:     `{"Cmd":["/bin/sh"],"Entrypoint":null,"Env":null,"Labels":null,"OnBuild":null,"StopSignal":"SIGQUIT","User":"","Volumes":null,"WorkingDir":""}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			imgInspect := &image.InspectResponse{}
			if tc.cfg != nil {
				// Verify that fields that are set override the legacy values,
				// or appended if not part of the legacy values.
				imgInspect.Config = &dockerspec.DockerOCIImageConfig{
					ImageConfig: *tc.cfg,
				}
			}
			out, err := json.Marshal(&inspectCompatResponse{
				InspectResponse: imgInspect,
				legacyConfig:    tc.legacyConfig,
			})
			assert.NilError(t, err)

			var outMap struct{ Config json.RawMessage }
			err = json.Unmarshal(out, &outMap)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(string(outMap.Config), tc.expected))
		})
	}
}
