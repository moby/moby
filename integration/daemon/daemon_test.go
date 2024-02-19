package daemon // import "github.com/docker/docker/integration/daemon"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/process"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestConfigDaemonID(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows")

	_ = testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	defer d.Stop(t)

	d.Start(t, "--iptables=false")
	info := d.Info(t)
	assert.Check(t, info.ID != "")
	d.Stop(t)

	// Verify that (if present) the engine-id file takes precedence
	const engineID = "this-is-the-engine-id"
	idFile := filepath.Join(d.RootDir(), "engine-id")
	assert.Check(t, os.Remove(idFile))
	// Using 0644 to allow rootless daemons to read the file (ideally
	// we'd chown the file to have the remapped user as owner).
	err := os.WriteFile(idFile, []byte(engineID), 0o644)
	assert.NilError(t, err)

	d.Start(t, "--iptables=false")
	info = d.Info(t)
	assert.Equal(t, info.ID, engineID)
	d.Stop(t)
}

func TestDaemonConfigValidation(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows")
	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	dockerBinary, err := d.BinaryPath()
	assert.NilError(t, err)
	params := []string{"--validate", "--config-file"}

	dest := os.Getenv("DOCKER_INTEGRATION_DAEMON_DEST")
	if dest == "" {
		dest = os.Getenv("DEST")
	}
	testdata := filepath.Join(dest, "..", "..", "integration", "daemon", "testdata")

	const (
		validOut  = "configuration OK"
		failedOut = "unable to configure the Docker daemon with file"
	)

	tests := []struct {
		name        string
		args        []string
		expectedOut string
	}{
		{
			name:        "config with no content",
			args:        append(params, filepath.Join(testdata, "empty-config-1.json")),
			expectedOut: validOut,
		},
		{
			name:        "config with {}",
			args:        append(params, filepath.Join(testdata, "empty-config-2.json")),
			expectedOut: validOut,
		},
		{
			name:        "invalid config",
			args:        append(params, filepath.Join(testdata, "invalid-config-1.json")),
			expectedOut: failedOut,
		},
		{
			name:        "malformed config",
			args:        append(params, filepath.Join(testdata, "malformed-config.json")),
			expectedOut: failedOut,
		},
		{
			name:        "valid config",
			args:        append(params, filepath.Join(testdata, "valid-config-1.json")),
			expectedOut: validOut,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_ = testutil.StartSpan(ctx, t)
			cmd := exec.Command(dockerBinary, tc.args...)
			out, err := cmd.CombinedOutput()
			assert.Check(t, is.Contains(string(out), tc.expectedOut))
			if tc.expectedOut == failedOut {
				assert.ErrorContains(t, err, "", "expected an error, but got none")
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestConfigDaemonSeccompProfiles(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows")
	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	defer d.Stop(t)

	tests := []struct {
		doc             string
		profile         string
		expectedProfile string
	}{
		{
			doc:             "empty profile set",
			profile:         "",
			expectedProfile: config.SeccompProfileDefault,
		},
		{
			doc:             "default profile",
			profile:         config.SeccompProfileDefault,
			expectedProfile: config.SeccompProfileDefault,
		},
		{
			doc:             "unconfined profile",
			profile:         config.SeccompProfileUnconfined,
			expectedProfile: config.SeccompProfileUnconfined,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			_ = testutil.StartSpan(ctx, t)

			d.Start(t, "--seccomp-profile="+tc.profile)
			info := d.Info(t)
			assert.Assert(t, is.Contains(info.SecurityOptions, "name=seccomp,profile="+tc.expectedProfile))
			d.Stop(t)

			cfg := filepath.Join(d.RootDir(), "daemon.json")
			err := os.WriteFile(cfg, []byte(`{"seccomp-profile": "`+tc.profile+`"}`), 0o644)
			assert.NilError(t, err)

			d.Start(t, "--config-file", cfg)
			info = d.Info(t)
			assert.Assert(t, is.Contains(info.SecurityOptions, "name=seccomp,profile="+tc.expectedProfile))
			d.Stop(t)
		})
	}
}

func TestDaemonProxy(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows", "cannot start multiple daemons on windows")
	skip.If(t, os.Getenv("DOCKER_ROOTLESS") != "", "cannot connect to localhost proxy in rootless environment")

	// Don't setup OTEL here to avoid it hitting the HTTP proxy.
	ctx := context.Background()

	newProxy := func(rcvd *string, t *testing.T) *httptest.Server {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*rcvd = r.Host
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("OK"))
		}))
		t.Cleanup(s.Close)
		return s
	}

	const userPass = "myuser:mypassword@"

	// Configure proxy through env-vars
	t.Run("environment variables", func(t *testing.T) {
		t.Parallel()

		var received string
		proxyServer := newProxy(&received, t)

		d := daemon.New(t, daemon.WithEnvVars(
			"HTTP_PROXY="+proxyServer.URL,
			"HTTPS_PROXY="+proxyServer.URL,
			"NO_PROXY=example.com",
		))
		c := d.NewClientT(t)

		d.Start(t, "--iptables=false")
		defer d.Stop(t)

		info := d.Info(t)
		assert.Check(t, is.Equal(info.HTTPProxy, proxyServer.URL))
		assert.Check(t, is.Equal(info.HTTPSProxy, proxyServer.URL))
		assert.Check(t, is.Equal(info.NoProxy, "example.com"))

		_, err := c.ImagePull(ctx, "example.org:5000/some/image:latest", image.PullOptions{})
		assert.ErrorContains(t, err, "", "pulling should have failed")
		assert.Equal(t, received, "example.org:5000")

		// Test NoProxy: example.com should not hit the proxy, and "received" variable should not be changed.
		_, err = c.ImagePull(ctx, "example.com/some/image:latest", image.PullOptions{})
		assert.ErrorContains(t, err, "", "pulling should have failed")
		assert.Equal(t, received, "example.org:5000", "should not have used proxy")
	})

	// Configure proxy through command-line flags
	t.Run("command-line options", func(t *testing.T) {
		t.Parallel()

		var received string
		proxyServer := newProxy(&received, t)

		d := daemon.New(t, daemon.WithEnvVars(
			"HTTP_PROXY="+"http://"+userPass+"from-env-http.invalid",
			"http_proxy="+"http://"+userPass+"from-env-http.invalid",
			"HTTPS_PROXY="+"https://"+userPass+"myuser:mypassword@from-env-https-invalid",
			"https_proxy="+"https://"+userPass+"myuser:mypassword@from-env-https-invalid",
			"NO_PROXY=ignore.invalid",
			"no_proxy=ignore.invalid",
		))
		d.Start(t, "--iptables=false", "--http-proxy", proxyServer.URL, "--https-proxy", proxyServer.URL, "--no-proxy", "example.com")
		defer d.Stop(t)

		c := d.NewClientT(t)

		info := d.Info(t)
		assert.Check(t, is.Equal(info.HTTPProxy, proxyServer.URL))
		assert.Check(t, is.Equal(info.HTTPSProxy, proxyServer.URL))
		assert.Check(t, is.Equal(info.NoProxy, "example.com"))

		ok, _ := d.ScanLogsT(ctx, t, daemon.ScanLogsMatchAll(
			"overriding existing proxy variable with value from configuration",
			"http_proxy",
			"HTTP_PROXY",
			"https_proxy",
			"HTTPS_PROXY",
			"no_proxy",
			"NO_PROXY",
		))
		assert.Assert(t, ok)

		ok, logs := d.ScanLogsT(ctx, t, daemon.ScanLogsMatchString(userPass))
		assert.Assert(t, !ok, "logs should not contain the non-sanitized proxy URL: %s", logs)

		_, err := c.ImagePull(ctx, "example.org:5001/some/image:latest", image.PullOptions{})
		assert.ErrorContains(t, err, "", "pulling should have failed")
		assert.Equal(t, received, "example.org:5001")

		// Test NoProxy: example.com should not hit the proxy, and "received" variable should not be changed.
		_, err = c.ImagePull(ctx, "example.com/some/image:latest", image.PullOptions{})
		assert.ErrorContains(t, err, "", "pulling should have failed")
		assert.Equal(t, received, "example.org:5001", "should not have used proxy")
	})

	// Configure proxy through configuration file
	t.Run("configuration file", func(t *testing.T) {
		t.Parallel()

		var received string
		proxyServer := newProxy(&received, t)

		d := daemon.New(t, daemon.WithEnvVars(
			"HTTP_PROXY="+"http://"+userPass+"from-env-http.invalid",
			"http_proxy="+"http://"+userPass+"from-env-http.invalid",
			"HTTPS_PROXY="+"https://"+userPass+"myuser:mypassword@from-env-https-invalid",
			"https_proxy="+"https://"+userPass+"myuser:mypassword@from-env-https-invalid",
			"NO_PROXY=ignore.invalid",
			"no_proxy=ignore.invalid",
		))
		c := d.NewClientT(t)

		configFile := filepath.Join(d.RootDir(), "daemon.json")
		configJSON := fmt.Sprintf(`{"proxies":{"http-proxy":%[1]q, "https-proxy": %[1]q, "no-proxy": "example.com"}}`, proxyServer.URL)
		assert.NilError(t, os.WriteFile(configFile, []byte(configJSON), 0o644))

		d.Start(t, "--iptables=false", "--config-file", configFile)
		defer d.Stop(t)

		info := d.Info(t)
		assert.Check(t, is.Equal(info.HTTPProxy, proxyServer.URL))
		assert.Check(t, is.Equal(info.HTTPSProxy, proxyServer.URL))
		assert.Check(t, is.Equal(info.NoProxy, "example.com"))

		d.ScanLogsT(ctx, t, daemon.ScanLogsMatchAll(
			"overriding existing proxy variable with value from configuration",
			"http_proxy",
			"HTTP_PROXY",
			"https_proxy",
			"HTTPS_PROXY",
			"no_proxy",
			"NO_PROXY",
		))

		_, err := c.ImagePull(ctx, "example.org:5002/some/image:latest", image.PullOptions{})
		assert.ErrorContains(t, err, "", "pulling should have failed")
		assert.Equal(t, received, "example.org:5002")

		// Test NoProxy: example.com should not hit the proxy, and "received" variable should not be changed.
		_, err = c.ImagePull(ctx, "example.com/some/image:latest", image.PullOptions{})
		assert.ErrorContains(t, err, "", "pulling should have failed")
		assert.Equal(t, received, "example.org:5002", "should not have used proxy")
	})

	// Conflicting options (passed both through command-line options and config file)
	t.Run("conflicting options", func(t *testing.T) {
		const (
			proxyRawURL = "https://" + userPass + "example.org"
			proxyURL    = "https://xxxxx:xxxxx@example.org"
		)

		d := daemon.New(t)

		configFile := filepath.Join(d.RootDir(), "daemon.json")
		configJSON := fmt.Sprintf(`{"proxies":{"http-proxy":%[1]q, "https-proxy": %[1]q, "no-proxy": "example.com"}}`, proxyRawURL)
		assert.NilError(t, os.WriteFile(configFile, []byte(configJSON), 0o644))

		err := d.StartWithError("--http-proxy", proxyRawURL, "--https-proxy", proxyRawURL, "--no-proxy", "example.com", "--config-file", configFile, "--validate")
		assert.ErrorContains(t, err, "daemon exited during startup")

		expected := fmt.Sprintf(
			`the following directives are specified both as a flag and in the configuration file: http-proxy: (from flag: %[1]s, from file: %[1]s), https-proxy: (from flag: %[1]s, from file: %[1]s), no-proxy: (from flag: example.com, from file: example.com)`,
			proxyURL,
		)
		poll.WaitOn(t, d.PollCheckLogs(ctx, daemon.ScanLogsMatchString(expected)))
	})

	// Make sure values are sanitized when reloading the daemon-config
	t.Run("reload sanitized", func(t *testing.T) {
		t.Parallel()

		const (
			proxyRawURL = "https://" + userPass + "example.org"
			proxyURL    = "https://xxxxx:xxxxx@example.org"
		)

		d := daemon.New(t)
		d.Start(t, "--iptables=false", "--http-proxy", proxyRawURL, "--https-proxy", proxyRawURL, "--no-proxy", "example.com")
		defer d.Stop(t)
		err := d.Signal(syscall.SIGHUP)
		assert.NilError(t, err)

		poll.WaitOn(t, d.PollCheckLogs(ctx, daemon.ScanLogsMatchAll("Reloaded configuration:", proxyURL)))

		ok, logs := d.ScanLogsT(ctx, t, daemon.ScanLogsMatchString(userPass))
		assert.Assert(t, !ok, "logs should not contain the non-sanitized proxy URL: %s", logs)
	})
}

func TestLiveRestore(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows", "cannot start multiple daemons on windows")
	_ = testutil.StartSpan(baseContext, t)

	t.Run("volume references", testLiveRestoreVolumeReferences)
	t.Run("autoremove", testLiveRestoreAutoRemove)
}

func testLiveRestoreAutoRemove(t *testing.T) {
	skip.If(t, testEnv.IsRootless(), "restarted rootless daemon will have a new process namespace")

	t.Parallel()
	ctx := testutil.StartSpan(baseContext, t)

	run := func(t *testing.T) (*daemon.Daemon, func(), string) {
		d := daemon.New(t)
		d.StartWithBusybox(ctx, t, "--live-restore", "--iptables=false")
		t.Cleanup(func() {
			d.Stop(t)
			d.Cleanup(t)
		})

		tmpDir := t.TempDir()

		apiClient := d.NewClientT(t)

		cID := container.Run(ctx, t, apiClient,
			container.WithBind(tmpDir, "/v"),
			// Run until a 'stop' file is created.
			container.WithCmd("sh", "-c", "while [ ! -f /v/stop ]; do sleep 0.1; done"),
			container.WithAutoRemove)
		t.Cleanup(func() { apiClient.ContainerRemove(ctx, cID, containertypes.RemoveOptions{Force: true}) })
		finishContainer := func() {
			file, err := os.Create(filepath.Join(tmpDir, "stop"))
			assert.NilError(t, err, "Failed to create 'stop' file")
			file.Close()
		}
		return d, finishContainer, cID
	}

	t.Run("engine restart shouldnt kill alive containers", func(t *testing.T) {
		d, finishContainer, cID := run(t)

		d.Restart(t, "--live-restore", "--iptables=false")

		apiClient := d.NewClientT(t)
		_, err := apiClient.ContainerInspect(ctx, cID)
		assert.NilError(t, err, "Container shouldn't be removed after engine restart")

		finishContainer()

		poll.WaitOn(t, container.IsRemoved(ctx, apiClient, cID))
	})
	t.Run("engine restart should remove containers that exited", func(t *testing.T) {
		d, finishContainer, cID := run(t)

		apiClient := d.NewClientT(t)

		// Get PID of the container process.
		inspect, err := apiClient.ContainerInspect(ctx, cID)
		assert.NilError(t, err)
		pid := inspect.State.Pid

		d.Stop(t)

		finishContainer()
		poll.WaitOn(t, process.NotAlive(pid))

		d.Start(t, "--live-restore", "--iptables=false")

		poll.WaitOn(t, container.IsRemoved(ctx, apiClient, cID))
	})
}

func testLiveRestoreVolumeReferences(t *testing.T) {
	t.Parallel()
	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "--live-restore", "--iptables=false")
	defer func() {
		d.Stop(t)
		d.Cleanup(t)
	}()

	c := d.NewClientT(t)

	runTest := func(t *testing.T, policy containertypes.RestartPolicyMode) {
		t.Run(string(policy), func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			volName := "test-live-restore-volume-references-" + string(policy)
			_, err := c.VolumeCreate(ctx, volume.CreateOptions{Name: volName})
			assert.NilError(t, err)

			// Create a container that uses the volume
			m := mount.Mount{
				Type:   mount.TypeVolume,
				Source: volName,
				Target: "/foo",
			}
			cID := container.Run(ctx, t, c, container.WithMount(m), container.WithCmd("top"), container.WithRestartPolicy(policy))
			defer c.ContainerRemove(ctx, cID, containertypes.RemoveOptions{Force: true})

			// Stop the daemon
			d.Restart(t, "--live-restore", "--iptables=false")

			// Try to remove the volume
			err = c.VolumeRemove(ctx, volName, false)
			assert.ErrorContains(t, err, "volume is in use")

			_, err = c.VolumeInspect(ctx, volName)
			assert.NilError(t, err)
		})
	}

	t.Run("restartPolicy", func(t *testing.T) {
		runTest(t, containertypes.RestartPolicyAlways)
		runTest(t, containertypes.RestartPolicyUnlessStopped)
		runTest(t, containertypes.RestartPolicyOnFailure)
		runTest(t, containertypes.RestartPolicyDisabled)
	})

	// Make sure that the local volume driver's mount ref count is restored
	// Addresses https://github.com/moby/moby/issues/44422
	t.Run("local volume with mount options", func(t *testing.T) {
		ctx := testutil.StartSpan(ctx, t)
		v, err := c.VolumeCreate(ctx, volume.CreateOptions{
			Driver: "local",
			Name:   "test-live-restore-volume-references-local",
			DriverOpts: map[string]string{
				"type":   "tmpfs",
				"device": "tmpfs",
			},
		})
		assert.NilError(t, err)
		m := mount.Mount{
			Type:   mount.TypeVolume,
			Source: v.Name,
			Target: "/foo",
		}

		const testContent = "hello"
		cID := container.Run(ctx, t, c, container.WithMount(m), container.WithCmd("sh", "-c", "echo "+testContent+">>/foo/test.txt; sleep infinity"))
		defer c.ContainerRemove(ctx, cID, containertypes.RemoveOptions{Force: true})

		// Wait until container creates a file in the volume.
		poll.WaitOn(t, func(t poll.LogT) poll.Result {
			stat, err := c.ContainerStatPath(ctx, cID, "/foo/test.txt")
			if err != nil {
				if errdefs.IsNotFound(err) {
					return poll.Continue("file doesn't yet exist")
				}
				return poll.Error(err)
			}

			if int(stat.Size) != len(testContent)+1 {
				return poll.Error(fmt.Errorf("unexpected test file size: %d", stat.Size))
			}

			return poll.Success()
		})

		d.Restart(t, "--live-restore", "--iptables=false")

		// Try to remove the volume
		// This should fail since its used by a container
		err = c.VolumeRemove(ctx, v.Name, false)
		assert.ErrorContains(t, err, "volume is in use")

		t.Run("volume still mounted", func(t *testing.T) {
			skip.If(t, testEnv.IsRootless(), "restarted rootless daemon has a new mount namespace and it won't have the previous mounts")

			// Check if a new container with the same volume has access to the previous content.
			// This fails if the volume gets unmounted at startup.
			cID2 := container.Run(ctx, t, c, container.WithMount(m), container.WithCmd("cat", "/foo/test.txt"))
			defer c.ContainerRemove(ctx, cID2, containertypes.RemoveOptions{Force: true})

			poll.WaitOn(t, container.IsStopped(ctx, c, cID2))

			inspect, err := c.ContainerInspect(ctx, cID2)
			if assert.Check(t, err) {
				assert.Check(t, is.Equal(inspect.State.ExitCode, 0), "volume doesn't have the same file")
			}

			logs, err := c.ContainerLogs(ctx, cID2, containertypes.LogsOptions{ShowStdout: true})
			assert.NilError(t, err)
			defer logs.Close()

			var stdoutBuf bytes.Buffer
			_, err = stdcopy.StdCopy(&stdoutBuf, io.Discard, logs)
			assert.NilError(t, err)

			assert.Check(t, is.Equal(strings.TrimSpace(stdoutBuf.String()), testContent))
		})

		// Remove that container which should free the references in the volume
		err = c.ContainerRemove(ctx, cID, containertypes.RemoveOptions{Force: true})
		assert.NilError(t, err)

		// Now we should be able to remove the volume
		err = c.VolumeRemove(ctx, v.Name, false)
		assert.NilError(t, err)
	})

	// Make sure that we don't panic if the container has bind-mounts
	// (which should not be "restored")
	// Regression test for https://github.com/moby/moby/issues/45898
	t.Run("container with bind-mounts", func(t *testing.T) {
		ctx := testutil.StartSpan(ctx, t)
		m := mount.Mount{
			Type:   mount.TypeBind,
			Source: os.TempDir(),
			Target: "/foo",
		}
		cID := container.Run(ctx, t, c, container.WithMount(m), container.WithCmd("top"))
		defer c.ContainerRemove(ctx, cID, containertypes.RemoveOptions{Force: true})

		d.Restart(t, "--live-restore", "--iptables=false")

		err := c.ContainerRemove(ctx, cID, containertypes.RemoveOptions{Force: true})
		assert.NilError(t, err)
	})
}

func TestDaemonDefaultBridgeWithFixedCidrButNoBip(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows")

	ctx := testutil.StartSpan(baseContext, t)

	bridgeName := "ext-bridge1"
	d := daemon.New(t, daemon.WithEnvVars("DOCKER_TEST_CREATE_DEFAULT_BRIDGE="+bridgeName))
	defer func() {
		d.Stop(t)
		d.Cleanup(t)
	}()

	defer func() {
		// No need to clean up when running this test in rootless mode, as the
		// interface is deleted when the daemon is stopped and the netns
		// reclaimed by the kernel.
		if !testEnv.IsRootless() {
			deleteInterface(t, bridgeName)
		}
	}()
	d.StartWithBusybox(ctx, t, "--bridge", bridgeName, "--fixed-cidr", "192.168.130.0/24")
}

func deleteInterface(t *testing.T, ifName string) {
	icmd.RunCommand("ip", "link", "delete", ifName).Assert(t, icmd.Success)
	icmd.RunCommand("iptables", "-t", "nat", "--flush").Assert(t, icmd.Success)
	icmd.RunCommand("iptables", "--flush").Assert(t, icmd.Success)
}
