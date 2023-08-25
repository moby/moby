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
	d := daemon.New(t, daemon.WithExperimental())
	d.StartWithBusybox(ctx, t, "--cdi-spec-dir="+filepath.Join(cwd, "testdata", "cdi"))
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
		experimental            bool
		specDirs                []string
		expectedInfoCDISpecDirs []string
	}{
		{
			description:             "experimental no spec dirs specified returns default",
			experimental:            true,
			specDirs:                nil,
			expectedInfoCDISpecDirs: []string{"/etc/cdi", "/var/run/cdi"},
		},
		{
			description:             "experimental specified spec dirs are returned",
			experimental:            true,
			specDirs:                []string{"/foo/bar", "/baz/qux"},
			expectedInfoCDISpecDirs: []string{"/foo/bar", "/baz/qux"},
		},
		{
			description:             "experimental empty string as spec dir returns empty slice",
			experimental:            true,
			specDirs:                []string{""},
			expectedInfoCDISpecDirs: []string{},
		},
		{
			description:             "experimental empty config option returns empty slice",
			experimental:            true,
			config:                  map[string]interface{}{"cdi-spec-dirs": []string{}},
			expectedInfoCDISpecDirs: []string{},
		},
		{
			description:             "non-experimental no spec dirs specified returns empty slice",
			experimental:            false,
			specDirs:                nil,
			expectedInfoCDISpecDirs: []string{},
		},
		{
			description:             "non-experimental specified spec dirs returns empty slice",
			experimental:            false,
			specDirs:                []string{"/foo/bar", "/baz/qux"},
			expectedInfoCDISpecDirs: []string{},
		},
		{
			description:             "non-experimental empty string as spec dir returns empty slice",
			experimental:            false,
			specDirs:                []string{""},
			expectedInfoCDISpecDirs: []string{},
		},
		{
			description:             "non-experimental empty config option returns empty slice",
			experimental:            false,
			config:                  map[string]interface{}{"cdi-spec-dirs": []string{}},
			expectedInfoCDISpecDirs: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			var opts []daemon.Option
			if tc.experimental {
				opts = append(opts, daemon.WithExperimental())
			}
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
