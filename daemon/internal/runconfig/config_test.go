package runconfig

import (
	"bytes"
	"encoding/json"
	"os"
	"runtime"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/pkg/sysinfo"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

/*
TestDecodeCreateRequest validates unmarshaling a container config fixture.

Fixture created using;

	docker run -it \
	    --cap-add=NET_ADMIN \
	    --cap-drop=MKNOD \
	    --cpu-shares=512 \
	    --cpuset-cpus=0,1 \
	    --dns=8.8.8.8 \
	    --entrypoint bash \
	    --label com.example.license=GPL \
	    --label com.example.vendor=Acme \
	    --label com.example.version=1.0 \
	    --link=redis3:redis \
	    --log-driver=json-file \
	    --mac-address="12:34:56:78:9a:bc" \
	    --memory=4M \
	    --network=bridge \
	    --volumes-from=other:ro \
	    --volumes-from=parent \
	    -p=11022:22/tcp \
	    -v /tmp \
	    -v /tmp:/tmp \
	    ubuntu date
*/
func TestDecodeCreateRequest(t *testing.T) {
	type testCase struct {
		doc        string
		imgName    string
		fixture    string
		entrypoint []string
	}

	tests := []testCase{
		{
			doc:        "API 1.24 windows",
			imgName:    "windows",
			fixture:    "fixtures/windows/container_config_1_24.json",
			entrypoint: []string{"cmd"},
		},
		{
			doc:        "API 1.24 unix",
			imgName:    "ubuntu",
			fixture:    "fixtures/unix/container_config_1_24.json",
			entrypoint: []string{"bash"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			b, err := os.ReadFile(tc.fixture)
			assert.NilError(t, err)

			req, err := DecodeCreateRequest(bytes.NewReader(b), sysinfo.New())
			assert.NilError(t, err)

			assert.Check(t, is.Equal(req.Image, tc.imgName))
			assert.Check(t, is.DeepEqual(req.Entrypoint, tc.entrypoint))

			var expected int64 = 4194304
			assert.Check(t, is.Equal(req.HostConfig.Memory, expected))
		})
	}
}

// TestDecodeCreateRequestIsolation validates isolation passed
// to the daemon in the hostConfig structure. Note this is platform specific
// as to what level of container isolation is supported.
func TestDecodeCreateRequestIsolation(t *testing.T) {
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
				Config: &container.Config{},
				HostConfig: &container.HostConfig{
					Isolation: container.Isolation(tc.isolation),
				},
			})
			assert.NilError(t, err)

			req, err := DecodeCreateRequest(bytes.NewReader(b), sysinfo.New())
			if tc.invalid {
				assert.Check(t, is.ErrorContains(err, tc.expectedErr))
				assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
				assert.Check(t, is.DeepEqual(req, container.CreateRequest{}))
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestDecodeCreateRequestPrivileged(t *testing.T) {
	requestJSON, err := json.Marshal(container.CreateRequest{
		Config:     &container.Config{},
		HostConfig: &container.HostConfig{Privileged: true},
	})
	assert.NilError(t, err)

	req, err := DecodeCreateRequest(bytes.NewReader(requestJSON), sysinfo.New())
	if runtime.GOOS == "windows" {
		const expected = "invalid option: privileged mode is not supported for Windows containers"
		assert.Check(t, is.Error(err, expected))
		assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
		assert.Check(t, is.DeepEqual(req, container.CreateRequest{}))
	} else {
		assert.NilError(t, err)
	}
}
