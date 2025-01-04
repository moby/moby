package runconfig // import "github.com/docker/docker/runconfig"

import (
	"bytes"
	"encoding/json"
	"os"
	"runtime"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/pkg/sysinfo"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type f struct {
	file       string
	entrypoint strslice.StrSlice
}

func TestDecodeContainerConfig(t *testing.T) {
	var (
		tests   []f
		imgName string
	)

	// FIXME (thaJeztah): update fixtures for more current versions.
	if runtime.GOOS != "windows" {
		imgName = "ubuntu"
		tests = []f{
			{"fixtures/unix/container_config_1_19.json", strslice.StrSlice{"bash"}},
		}
	} else {
		imgName = "windows"
		tests = []f{
			{"fixtures/windows/container_config_1_19.json", strslice.StrSlice{"cmd"}},
		}
	}

	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			b, err := os.ReadFile(tc.file)
			if err != nil {
				t.Fatal(err)
			}

			c, h, _, err := decodeContainerConfig(bytes.NewReader(b), sysinfo.New())
			if err != nil {
				t.Fatal(err)
			}

			if c.Image != imgName {
				t.Fatalf("Expected %s image, found %s", imgName, c.Image)
			}

			if len(c.Entrypoint) != len(tc.entrypoint) {
				t.Fatalf("Expected %v, found %v", tc.entrypoint, c.Entrypoint)
			}

			if h != nil && h.Memory != 1000 {
				t.Fatalf("Expected memory to be 1000, found %d", h.Memory)
			}
		})
	}
}

// TestDecodeContainerConfigIsolation validates isolation passed
// to the daemon in the hostConfig structure. Note this is platform specific
// as to what level of container isolation is supported.
func TestDecodeContainerConfigIsolation(t *testing.T) {
	tests := []struct {
		isolation   string
		invalid     bool
		expectedErr string
	}{
		{
			isolation: "",
		},
		{
			isolation: "default",
		},
		{
			isolation:   "invalid",
			invalid:     true,
			expectedErr: `Invalid isolation: "invalid"`,
		},
		{
			isolation:   "process",
			invalid:     runtime.GOOS != "windows",
			expectedErr: `Invalid isolation: "process"`,
		},
		{
			isolation:   "hyperv",
			invalid:     runtime.GOOS != "windows",
			expectedErr: `Invalid isolation: "hyperv"`,
		},
	}
	for _, tc := range tests {
		t.Run("isolation="+tc.isolation, func(t *testing.T) {
			// TODO(thaJeztah): consider using fixtures for the JSON requests so that we don't depend on current implementations.
			b, err := json.Marshal(container.CreateRequest{
				HostConfig: &container.HostConfig{
					Isolation: container.Isolation(tc.isolation),
				},
			})
			assert.NilError(t, err)

			_, _, _, err = decodeContainerConfig(bytes.NewReader(b), sysinfo.New())
			if tc.invalid {
				assert.Check(t, is.ErrorContains(err, tc.expectedErr))
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestDecodeContainerConfigPrivileged(t *testing.T) {
	requestJSON, err := json.Marshal(container.CreateRequest{HostConfig: &container.HostConfig{Privileged: true}})
	assert.NilError(t, err)

	_, _, _, err = decodeContainerConfig(bytes.NewReader(requestJSON), sysinfo.New())
	if runtime.GOOS == "windows" {
		const expected = "Windows does not support privileged mode"
		assert.Check(t, is.Error(err, expected))
	} else {
		assert.NilError(t, err)
	}
}
