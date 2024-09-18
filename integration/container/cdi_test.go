package container // import "github.com/docker/docker/integration/container"

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestCreateWithCDIDevices(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux", "CDI devices are only supported on Linux")
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run cdi tests with a remote daemon")

	ctx := testutil.StartSpan(baseContext, t)

	cwd, err := os.Getwd()
	assert.NilError(t, err)
	configPath := filepath.Join(cwd, "daemon.json")
	err = os.WriteFile(configPath, []byte(`{"features": {"cdi": true}}`), 0o644)
	defer os.Remove(configPath)
	assert.NilError(t, err)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "--config-file", configPath, "--cdi-spec-dir="+filepath.Join(cwd, "testdata", "cdi"))
	defer d.Stop(t)

	apiClient := d.NewClientT(t)

	id := container.Run(ctx, t, apiClient,
		container.WithCmd("/bin/sh", "-c", "env"),
		container.WithCDIDevices("vendor1.com/device=foo"),
	)
	defer apiClient.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: true})

	inspect, err := apiClient.ContainerInspect(ctx, id)
	assert.NilError(t, err)

	expectedRequests := []containertypes.DeviceRequest{
		{
			Driver:    "cdi",
			DeviceIDs: []string{"vendor1.com/device=foo"},
		},
	}
	assert.Check(t, is.DeepEqual(inspect.HostConfig.DeviceRequests, expectedRequests))

	reader, err := apiClient.ContainerLogs(ctx, id, containertypes.LogsOptions{
		ShowStdout: true,
	})
	assert.NilError(t, err)

	actualStdout := new(bytes.Buffer)
	actualStderr := io.Discard
	_, err = stdcopy.StdCopy(actualStdout, actualStderr, reader)
	assert.NilError(t, err)

	outlines := strings.Split(actualStdout.String(), "\n")
	assert.Assert(t, is.Contains(outlines, "FOO=injected"))
}

func TestCDISpecDirsAreInSystemInfo(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows") // d.Start fails on Windows with `protocol not available`
	// TODO: This restriction can be relaxed with https://github.com/moby/moby/pull/46158
	skip.If(t, testEnv.IsRootless, "the t.TempDir test creates a folder with incorrect permissions for rootless")

	testCases := []struct {
		description             string
		config                  map[string]interface{}
		specDirs                []string
		expectedInfoCDISpecDirs []string
	}{
		{
			description:             "CDI enabled with no spec dirs specified returns default",
			config:                  map[string]interface{}{"features": map[string]bool{"cdi": true}},
			specDirs:                nil,
			expectedInfoCDISpecDirs: []string{"/etc/cdi", "/var/run/cdi"},
		},
		{
			description:             "CDI enabled with specified spec dirs are returned",
			config:                  map[string]interface{}{"features": map[string]bool{"cdi": true}},
			specDirs:                []string{"/foo/bar", "/baz/qux"},
			expectedInfoCDISpecDirs: []string{"/foo/bar", "/baz/qux"},
		},
		{
			description:             "CDI enabled with empty string as spec dir returns empty slice",
			config:                  map[string]interface{}{"features": map[string]bool{"cdi": true}},
			specDirs:                []string{""},
			expectedInfoCDISpecDirs: []string{},
		},
		{
			description:             "CDI enabled with empty config option returns empty slice",
			config:                  map[string]interface{}{"features": map[string]bool{"cdi": true}, "cdi-spec-dirs": []string{}},
			expectedInfoCDISpecDirs: []string{},
		},
		{
			description:             "CDI disabled with no spec dirs specified returns empty slice",
			specDirs:                nil,
			expectedInfoCDISpecDirs: []string{},
		},
		{
			description:             "CDI disabled with specified spec dirs returns empty slice",
			specDirs:                []string{"/foo/bar", "/baz/qux"},
			expectedInfoCDISpecDirs: []string{},
		},
		{
			description:             "CDI disabled with empty string as spec dir returns empty slice",
			specDirs:                []string{""},
			expectedInfoCDISpecDirs: []string{},
		},
		{
			description:             "CDI disabled with empty config option returns empty slice",
			config:                  map[string]interface{}{"cdi-spec-dirs": []string{}},
			expectedInfoCDISpecDirs: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			var opts []daemon.Option
			d := daemon.New(t, opts...)

			var args []string
			for _, specDir := range tc.specDirs {
				args = append(args, "--cdi-spec-dir="+specDir)
			}
			if tc.config != nil {
				configPath := filepath.Join(t.TempDir(), "daemon.json")

				configFile, err := os.Create(configPath)
				assert.NilError(t, err)
				defer configFile.Close()

				err = json.NewEncoder(configFile).Encode(tc.config)
				assert.NilError(t, err)

				args = append(args, "--config-file="+configPath)
			}
			d.Start(t, args...)
			defer d.Stop(t)

			info := d.Info(t)

			assert.Check(t, is.DeepEqual(tc.expectedInfoCDISpecDirs, info.CDISpecDirs))
		})
	}
}
