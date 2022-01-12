package daemon // import "github.com/docker/docker/integration/daemon"

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/env"
	"gotest.tools/v3/skip"
)

func TestConfigDaemonLibtrustID(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows")

	d := daemon.New(t)
	defer d.Stop(t)

	trustKey := filepath.Join(d.RootDir(), "key.json")
	err := os.WriteFile(trustKey, []byte(`{"crv":"P-256","d":"dm28PH4Z4EbyUN8L0bPonAciAQa1QJmmyYd876mnypY","kid":"WTJ3:YSIP:CE2E:G6KJ:PSBD:YX2Y:WEYD:M64G:NU2V:XPZV:H2CR:VLUB","kty":"EC","x":"Mh5-JINSjaa_EZdXDttri255Z5fbCEOTQIZjAcScFTk","y":"eUyuAjfxevb07hCCpvi4Zi334Dy4GDWQvEToGEX4exQ"}`), 0644)
	assert.NilError(t, err)

	config := filepath.Join(d.RootDir(), "daemon.json")
	err = os.WriteFile(config, []byte(`{"deprecated-key-path": "`+trustKey+`"}`), 0644)
	assert.NilError(t, err)

	d.Start(t, "--config-file", config)
	info := d.Info(t)
	assert.Equal(t, info.ID, "WTJ3:YSIP:CE2E:G6KJ:PSBD:YX2Y:WEYD:M64G:NU2V:XPZV:H2CR:VLUB")
}

func TestDaemonConfigValidation(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows")

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
			d.Start(t, "--seccomp-profile="+tc.profile)
			info := d.Info(t)
			assert.Assert(t, is.Contains(info.SecurityOptions, "name=seccomp,profile="+tc.expectedProfile))
			d.Stop(t)

			cfg := filepath.Join(d.RootDir(), "daemon.json")
			err := os.WriteFile(cfg, []byte(`{"seccomp-profile": "`+tc.profile+`"}`), 0644)
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

	var received string
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Host
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("OK"))
	}))
	defer proxyServer.Close()

	const userPass = "myuser:mypassword@"

	// Configure proxy through env-vars
	t.Run("environment variables", func(t *testing.T) {
		defer env.Patch(t, "HTTP_PROXY", proxyServer.URL)()
		defer env.Patch(t, "HTTPS_PROXY", proxyServer.URL)()
		defer env.Patch(t, "NO_PROXY", "example.com")()

		d := daemon.New(t)
		c := d.NewClientT(t)
		defer func() { _ = c.Close() }()
		ctx := context.Background()
		d.Start(t)

		_, err := c.ImagePull(ctx, "example.org:5000/some/image:latest", types.ImagePullOptions{})
		assert.ErrorContains(t, err, "", "pulling should have failed")
		assert.Equal(t, received, "example.org:5000")

		// Test NoProxy: example.com should not hit the proxy, and "received" variable should not be changed.
		_, err = c.ImagePull(ctx, "example.com/some/image:latest", types.ImagePullOptions{})
		assert.ErrorContains(t, err, "", "pulling should have failed")
		assert.Equal(t, received, "example.org:5000", "should not have used proxy")

		info := d.Info(t)
		assert.Equal(t, info.HTTPProxy, proxyServer.URL)
		assert.Equal(t, info.HTTPSProxy, proxyServer.URL)
		assert.Equal(t, info.NoProxy, "example.com")
		d.Stop(t)
	})

	// Configure proxy through command-line flags
	t.Run("command-line options", func(t *testing.T) {
		defer env.Patch(t, "HTTP_PROXY", "http://"+userPass+"from-env-http.invalid")()
		defer env.Patch(t, "http_proxy", "http://"+userPass+"from-env-http.invalid")()
		defer env.Patch(t, "HTTPS_PROXY", "https://"+userPass+"myuser:mypassword@from-env-https.invalid")()
		defer env.Patch(t, "https_proxy", "https://"+userPass+"myuser:mypassword@from-env-https.invalid")()
		defer env.Patch(t, "NO_PROXY", "ignore.invalid")()
		defer env.Patch(t, "no_proxy", "ignore.invalid")()

		d := daemon.New(t)
		d.Start(t, "--http-proxy", proxyServer.URL, "--https-proxy", proxyServer.URL, "--no-proxy", "example.com")

		logs, err := d.ReadLogFile()
		assert.NilError(t, err)
		assert.Assert(t, is.Contains(string(logs), "overriding existing proxy variable with value from configuration"))
		for _, v := range []string{"http_proxy", "HTTP_PROXY", "https_proxy", "HTTPS_PROXY", "no_proxy", "NO_PROXY"} {
			assert.Assert(t, is.Contains(string(logs), "name="+v))
			assert.Assert(t, !strings.Contains(string(logs), userPass), "logs should not contain the non-sanitized proxy URL: %s", string(logs))
		}

		c := d.NewClientT(t)
		defer func() { _ = c.Close() }()
		ctx := context.Background()

		_, err = c.ImagePull(ctx, "example.org:5001/some/image:latest", types.ImagePullOptions{})
		assert.ErrorContains(t, err, "", "pulling should have failed")
		assert.Equal(t, received, "example.org:5001")

		// Test NoProxy: example.com should not hit the proxy, and "received" variable should not be changed.
		_, err = c.ImagePull(ctx, "example.com/some/image:latest", types.ImagePullOptions{})
		assert.ErrorContains(t, err, "", "pulling should have failed")
		assert.Equal(t, received, "example.org:5001", "should not have used proxy")

		info := d.Info(t)
		assert.Equal(t, info.HTTPProxy, proxyServer.URL)
		assert.Equal(t, info.HTTPSProxy, proxyServer.URL)
		assert.Equal(t, info.NoProxy, "example.com")

		d.Stop(t)
	})

	// Configure proxy through configuration file
	t.Run("configuration file", func(t *testing.T) {
		defer env.Patch(t, "HTTP_PROXY", "http://"+userPass+"from-env-http.invalid")()
		defer env.Patch(t, "http_proxy", "http://"+userPass+"from-env-http.invalid")()
		defer env.Patch(t, "HTTPS_PROXY", "https://"+userPass+"myuser:mypassword@from-env-https.invalid")()
		defer env.Patch(t, "https_proxy", "https://"+userPass+"myuser:mypassword@from-env-https.invalid")()
		defer env.Patch(t, "NO_PROXY", "ignore.invalid")()
		defer env.Patch(t, "no_proxy", "ignore.invalid")()

		d := daemon.New(t)
		c := d.NewClientT(t)
		defer func() { _ = c.Close() }()
		ctx := context.Background()

		configFile := filepath.Join(d.RootDir(), "daemon.json")
		configJSON := fmt.Sprintf(`{"http-proxy":%[1]q, "https-proxy": %[1]q, "no-proxy": "example.com"}`, proxyServer.URL)
		assert.NilError(t, os.WriteFile(configFile, []byte(configJSON), 0644))

		d.Start(t, "--config-file", configFile)

		logs, err := d.ReadLogFile()
		assert.NilError(t, err)
		assert.Assert(t, is.Contains(string(logs), "overriding existing proxy variable with value from configuration"))
		for _, v := range []string{"http_proxy", "HTTP_PROXY", "https_proxy", "HTTPS_PROXY", "no_proxy", "NO_PROXY"} {
			assert.Assert(t, is.Contains(string(logs), "name="+v))
			assert.Assert(t, !strings.Contains(string(logs), userPass), "logs should not contain the non-sanitized proxy URL: %s", string(logs))
		}

		_, err = c.ImagePull(ctx, "example.org:5002/some/image:latest", types.ImagePullOptions{})
		assert.ErrorContains(t, err, "", "pulling should have failed")
		assert.Equal(t, received, "example.org:5002")

		// Test NoProxy: example.com should not hit the proxy, and "received" variable should not be changed.
		_, err = c.ImagePull(ctx, "example.com/some/image:latest", types.ImagePullOptions{})
		assert.ErrorContains(t, err, "", "pulling should have failed")
		assert.Equal(t, received, "example.org:5002", "should not have used proxy")

		info := d.Info(t)
		assert.Equal(t, info.HTTPProxy, proxyServer.URL)
		assert.Equal(t, info.HTTPSProxy, proxyServer.URL)
		assert.Equal(t, info.NoProxy, "example.com")

		d.Stop(t)
	})

	// Conflicting options (passed both through command-line options and config file)
	t.Run("conflicting options", func(t *testing.T) {
		const (
			proxyRawURL = "https://" + userPass + "example.org"
			proxyURL    = "https://xxxxx:xxxxx@example.org"
		)

		d := daemon.New(t)

		configFile := filepath.Join(d.RootDir(), "daemon.json")
		configJSON := fmt.Sprintf(`{"http-proxy":%[1]q, "https-proxy": %[1]q, "no-proxy": "example.com"}`, proxyRawURL)
		assert.NilError(t, os.WriteFile(configFile, []byte(configJSON), 0644))

		err := d.StartWithError("--http-proxy", proxyRawURL, "--https-proxy", proxyRawURL, "--no-proxy", "example.com", "--config-file", configFile, "--validate")
		assert.ErrorContains(t, err, "daemon exited during startup")
		logs, err := d.ReadLogFile()
		assert.NilError(t, err)
		expected := fmt.Sprintf(
			`the following directives are specified both as a flag and in the configuration file: http-proxy: (from flag: %[1]s, from file: %[1]s), https-proxy: (from flag: %[1]s, from file: %[1]s), no-proxy: (from flag: example.com, from file: example.com)`,
			proxyURL,
		)
		assert.Assert(t, is.Contains(string(logs), expected))
	})

	// Make sure values are sanitized when reloading the daemon-config
	t.Run("reload sanitized", func(t *testing.T) {
		const (
			proxyRawURL = "https://" + userPass + "example.org"
			proxyURL    = "https://xxxxx:xxxxx@example.org"
		)

		d := daemon.New(t)
		d.Start(t, "--http-proxy", proxyRawURL, "--https-proxy", proxyRawURL, "--no-proxy", "example.com")
		defer d.Stop(t)
		err := d.Signal(syscall.SIGHUP)
		assert.NilError(t, err)

		logs, err := d.ReadLogFile()
		assert.NilError(t, err)

		// FIXME: there appears to ba a race condition, which causes ReadLogFile
		//        to not contain the full logs after signaling the daemon to reload,
		//        causing the test to fail here. As a workaround, check if we
		//        received the "reloaded" message after signaling, and only then
		//        check that it's sanitized properly. For more details on this
		//        issue, see https://github.com/moby/moby/pull/42835/files#r713120315
		if !strings.Contains(string(logs), "Reloaded configuration:") {
			t.Skip("Skipping test, because we did not find 'Reloaded configuration' in the logs")
		}

		assert.Assert(t, is.Contains(string(logs), proxyURL))
		assert.Assert(t, !strings.Contains(string(logs), userPass), "logs should not contain the non-sanitized proxy URL: %s", string(logs))
	})
}
