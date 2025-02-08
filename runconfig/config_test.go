package runconfig // import "github.com/docker/docker/runconfig"

import (
	"bytes"
	"encoding/json"
	"os"
	"runtime"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/sysinfo"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDecodeContainerConfig(t *testing.T) {
	type testCase struct {
		doc        string
		imgName    string
		fixture    string
		entrypoint []string
	}

	// FIXME (thaJeztah): update fixtures for more current versions.
	tests := []testCase{
		{
			doc:        "API 1.19 windows",
			imgName:    "windows",
			fixture:    "fixtures/windows/container_config_1_19.json",
			entrypoint: []string{"cmd"},
		},
		{
			doc:        "API 1.19 unix",
			imgName:    "ubuntu",
			fixture:    "fixtures/unix/container_config_1_19.json",
			entrypoint: []string{"bash"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			b, err := os.ReadFile(tc.fixture)
			assert.NilError(t, err)

			c, h, _, err := decodeContainerConfig(bytes.NewReader(b), sysinfo.New())
			assert.NilError(t, err)

			assert.Check(t, is.Equal(c.Image, tc.imgName))
			assert.Check(t, is.DeepEqual([]string(c.Entrypoint), tc.entrypoint))

			var expected int64 = 1000
			assert.Check(t, is.Equal(h.Memory, expected))
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
			expectedErr: `invalid isolation (invalid):`,
		},
		{
			isolation:   "process",
			invalid:     runtime.GOOS != "windows",
			expectedErr: `invalid isolation (process):`,
		},
		{
			isolation:   "hyperv",
			invalid:     runtime.GOOS != "windows",
			expectedErr: `invalid isolation (hyperv):`,
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

			cfg, hostConfig, nwConfig, err := decodeContainerConfig(bytes.NewReader(b), sysinfo.New())
			if tc.invalid {
				assert.Check(t, is.ErrorContains(err, tc.expectedErr))
				assert.Check(t, is.ErrorType(err, errdefs.IsInvalidParameter))
				assert.Check(t, is.Nil(cfg))
				assert.Check(t, is.Nil(hostConfig))
				assert.Check(t, is.Nil(nwConfig))
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestDecodeContainerConfigPrivileged(t *testing.T) {
	requestJSON, err := json.Marshal(container.CreateRequest{HostConfig: &container.HostConfig{Privileged: true}})
	assert.NilError(t, err)

	cfg, hostConfig, nwConfig, err := decodeContainerConfig(bytes.NewReader(requestJSON), sysinfo.New())
	if runtime.GOOS == "windows" {
		const expected = "invalid option: privileged mode is not supported for Windows containers"
		assert.Check(t, is.Error(err, expected))
		assert.Check(t, is.ErrorType(err, errdefs.IsInvalidParameter))
		assert.Check(t, is.Nil(cfg))
		assert.Check(t, is.Nil(hostConfig))
		assert.Check(t, is.Nil(nwConfig))
	} else {
		assert.NilError(t, err)
	}
}
