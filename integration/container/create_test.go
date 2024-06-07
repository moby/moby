package container // import "github.com/docker/docker/integration/container"

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/containerd/containerd"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	ctr "github.com/docker/docker/integration/internal/container"
	net "github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/oci"
	"github.com/docker/docker/testutil"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestCreateFailsWhenIdentifierDoesNotExist(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	testCases := []struct {
		doc           string
		image         string
		expectedError string
	}{
		{
			doc:           "image and tag",
			image:         "test456:v1",
			expectedError: "No such image: test456:v1",
		},
		{
			doc:           "image no tag",
			image:         "test456",
			expectedError: "No such image: test456",
		},
		{
			doc:           "digest",
			image:         "sha256:0cb40641836c461bc97c793971d84d758371ed682042457523e4ae701efeaaaa",
			expectedError: "No such image: sha256:0cb40641836c461bc97c793971d84d758371ed682042457523e4ae701efeaaaa",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.StartSpan(ctx, t)
			_, err := apiClient.ContainerCreate(ctx,
				&container.Config{Image: tc.image},
				&container.HostConfig{},
				&network.NetworkingConfig{},
				nil,
				"",
			)
			assert.Check(t, is.ErrorContains(err, tc.expectedError))
			assert.Check(t, errdefs.IsNotFound(err))
		})
	}
}

// TestCreateLinkToNonExistingContainer verifies that linking to a non-existing
// container returns an "invalid parameter" (400) status, and not the underlying
// "non exists" (404).
func TestCreateLinkToNonExistingContainer(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "legacy links are not supported on windows")
	ctx := setupTest(t)
	c := testEnv.APIClient()

	_, err := c.ContainerCreate(ctx,
		&container.Config{
			Image: "busybox",
		},
		&container.HostConfig{
			Links: []string{"no-such-container"},
		},
		&network.NetworkingConfig{},
		nil,
		"",
	)
	assert.Check(t, is.ErrorContains(err, "could not get container for no-such-container"))
	assert.Check(t, errdefs.IsInvalidParameter(err))
}

func TestCreateWithInvalidEnv(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	testCases := []struct {
		env           string
		expectedError string
	}{
		{
			env:           "",
			expectedError: "invalid environment variable:",
		},
		{
			env:           "=",
			expectedError: "invalid environment variable: =",
		},
		{
			env:           "=foo",
			expectedError: "invalid environment variable: =foo",
		},
	}

	for index, tc := range testCases {
		tc := tc
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			t.Parallel()
			ctx := testutil.StartSpan(ctx, t)
			_, err := apiClient.ContainerCreate(ctx,
				&container.Config{
					Image: "busybox",
					Env:   []string{tc.env},
				},
				&container.HostConfig{},
				&network.NetworkingConfig{},
				nil,
				"",
			)
			assert.Check(t, is.ErrorContains(err, tc.expectedError))
			assert.Check(t, errdefs.IsInvalidParameter(err))
		})
	}
}

// Test case for #30166 (target was not validated)
func TestCreateTmpfsMountsTarget(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()

	testCases := []struct {
		target        string
		expectedError string
	}{
		{
			target:        ".",
			expectedError: "mount path must be absolute",
		},
		{
			target:        "foo",
			expectedError: "mount path must be absolute",
		},
		{
			target:        "/",
			expectedError: "destination can't be '/'",
		},
		{
			target:        "//",
			expectedError: "destination can't be '/'",
		},
	}

	for _, tc := range testCases {
		_, err := apiClient.ContainerCreate(ctx,
			&container.Config{
				Image: "busybox",
			},
			&container.HostConfig{
				Tmpfs: map[string]string{tc.target: ""},
			},
			&network.NetworkingConfig{},
			nil,
			"",
		)
		assert.Check(t, is.ErrorContains(err, tc.expectedError))
		assert.Check(t, errdefs.IsInvalidParameter(err))
	}
}

func TestCreateWithCustomMaskedPaths(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	testCases := []struct {
		maskedPaths []string
		expected    []string
	}{
		{
			maskedPaths: []string{},
			expected:    []string{},
		},
		{
			maskedPaths: nil,
			expected:    oci.DefaultSpec().Linux.MaskedPaths,
		},
		{
			maskedPaths: []string{"/proc/kcore", "/proc/keys"},
			expected:    []string{"/proc/kcore", "/proc/keys"},
		},
	}

	checkInspect := func(t *testing.T, ctx context.Context, name string, expected []string) {
		_, b, err := apiClient.ContainerInspectWithRaw(ctx, name, false)
		assert.NilError(t, err)

		var inspectJSON map[string]interface{}
		err = json.Unmarshal(b, &inspectJSON)
		assert.NilError(t, err)

		cfg, ok := inspectJSON["HostConfig"].(map[string]interface{})
		assert.Check(t, is.Equal(true, ok), name)

		maskedPaths, ok := cfg["MaskedPaths"].([]interface{})
		assert.Check(t, is.Equal(true, ok), name)

		var mps = make([]string, 0, len(maskedPaths))
		for _, mp := range maskedPaths {
			mps = append(mps, mp.(string))
		}

		assert.DeepEqual(t, expected, mps)
	}

	// TODO: This should be using subtests

	for i, tc := range testCases {
		name := fmt.Sprintf("create-masked-paths-%d", i)
		config := container.Config{
			Image: "busybox",
			Cmd:   []string{"true"},
		}
		hc := container.HostConfig{}
		if tc.maskedPaths != nil {
			hc.MaskedPaths = tc.maskedPaths
		}

		// Create the container.
		c, err := apiClient.ContainerCreate(ctx,
			&config,
			&hc,
			&network.NetworkingConfig{},
			nil,
			name,
		)
		assert.NilError(t, err)

		checkInspect(t, ctx, name, tc.expected)

		// Start the container.
		err = apiClient.ContainerStart(ctx, c.ID, container.StartOptions{})
		assert.NilError(t, err)

		poll.WaitOn(t, ctr.IsInState(ctx, apiClient, c.ID, "exited"), poll.WithDelay(100*time.Millisecond))

		checkInspect(t, ctx, name, tc.expected)
	}
}

func TestCreateWithCustomReadonlyPaths(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	testCases := []struct {
		readonlyPaths []string
		expected      []string
	}{
		{
			readonlyPaths: []string{},
			expected:      []string{},
		},
		{
			readonlyPaths: nil,
			expected:      oci.DefaultSpec().Linux.ReadonlyPaths,
		},
		{
			readonlyPaths: []string{"/proc/asound", "/proc/bus"},
			expected:      []string{"/proc/asound", "/proc/bus"},
		},
	}

	checkInspect := func(t *testing.T, ctx context.Context, name string, expected []string) {
		_, b, err := apiClient.ContainerInspectWithRaw(ctx, name, false)
		assert.NilError(t, err)

		var inspectJSON map[string]interface{}
		err = json.Unmarshal(b, &inspectJSON)
		assert.NilError(t, err)

		cfg, ok := inspectJSON["HostConfig"].(map[string]interface{})
		assert.Check(t, is.Equal(true, ok), name)

		readonlyPaths, ok := cfg["ReadonlyPaths"].([]interface{})
		assert.Check(t, is.Equal(true, ok), name)

		var rops = make([]string, 0, len(readonlyPaths))
		for _, rop := range readonlyPaths {
			rops = append(rops, rop.(string))
		}
		assert.DeepEqual(t, expected, rops)
	}

	for i, tc := range testCases {
		name := fmt.Sprintf("create-readonly-paths-%d", i)
		config := container.Config{
			Image: "busybox",
			Cmd:   []string{"true"},
		}
		hc := container.HostConfig{}
		if tc.readonlyPaths != nil {
			hc.ReadonlyPaths = tc.readonlyPaths
		}

		// Create the container.
		c, err := apiClient.ContainerCreate(ctx,
			&config,
			&hc,
			&network.NetworkingConfig{},
			nil,
			name,
		)
		assert.NilError(t, err)

		checkInspect(t, ctx, name, tc.expected)

		// Start the container.
		err = apiClient.ContainerStart(ctx, c.ID, container.StartOptions{})
		assert.NilError(t, err)

		poll.WaitOn(t, ctr.IsInState(ctx, apiClient, c.ID, "exited"), poll.WithDelay(100*time.Millisecond))

		checkInspect(t, ctx, name, tc.expected)
	}
}

func TestCreateWithInvalidHealthcheckParams(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	testCases := []struct {
		doc           string
		interval      time.Duration
		timeout       time.Duration
		retries       int
		startPeriod   time.Duration
		startInterval time.Duration
		expectedErr   string
	}{
		{
			doc:         "test invalid Interval in Healthcheck: less than 0s",
			interval:    -10 * time.Millisecond,
			timeout:     time.Second,
			retries:     1000,
			expectedErr: fmt.Sprintf("Interval in Healthcheck cannot be less than %s", container.MinimumDuration),
		},
		{
			doc:         "test invalid Interval in Healthcheck: larger than 0s but less than 1ms",
			interval:    500 * time.Microsecond,
			timeout:     time.Second,
			retries:     1000,
			expectedErr: fmt.Sprintf("Interval in Healthcheck cannot be less than %s", container.MinimumDuration),
		},
		{
			doc:         "test invalid Timeout in Healthcheck: less than 1ms",
			interval:    time.Second,
			timeout:     -100 * time.Millisecond,
			retries:     1000,
			expectedErr: fmt.Sprintf("Timeout in Healthcheck cannot be less than %s", container.MinimumDuration),
		},
		{
			doc:         "test invalid Retries in Healthcheck: less than 0",
			interval:    time.Second,
			timeout:     time.Second,
			retries:     -10,
			expectedErr: "Retries in Healthcheck cannot be negative",
		},
		{
			doc:         "test invalid StartPeriod in Healthcheck: not 0 and less than 1ms",
			interval:    time.Second,
			timeout:     time.Second,
			retries:     1000,
			startPeriod: 100 * time.Microsecond,
			expectedErr: fmt.Sprintf("StartPeriod in Healthcheck cannot be less than %s", container.MinimumDuration),
		},
		{
			doc:           "test invalid StartInterval in Healthcheck: not 0 and less than 1ms",
			interval:      time.Second,
			timeout:       time.Second,
			retries:       1000,
			startPeriod:   time.Second,
			startInterval: 100 * time.Microsecond,
			expectedErr:   fmt.Sprintf("StartInterval in Healthcheck cannot be less than %s", container.MinimumDuration),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.StartSpan(ctx, t)
			cfg := container.Config{
				Image: "busybox",
				Healthcheck: &container.HealthConfig{
					Interval:      tc.interval,
					Timeout:       tc.timeout,
					Retries:       tc.retries,
					StartInterval: tc.startInterval,
				},
			}
			if tc.startPeriod != 0 {
				cfg.Healthcheck.StartPeriod = tc.startPeriod
			}

			resp, err := apiClient.ContainerCreate(ctx, &cfg, &container.HostConfig{}, nil, nil, "")
			assert.Check(t, is.Equal(len(resp.Warnings), 0))
			assert.Check(t, errdefs.IsInvalidParameter(err))
			assert.ErrorContains(t, err, tc.expectedErr)
		})
	}
}

// Make sure that anonymous volumes can be overwritten by tmpfs
// https://github.com/moby/moby/issues/40446
func TestCreateTmpfsOverrideAnonymousVolume(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "windows does not support tmpfs")
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	id := ctr.Create(ctx, t, apiClient,
		ctr.WithVolume("/foo"),
		ctr.WithTmpfs("/foo"),
		ctr.WithVolume("/bar"),
		ctr.WithTmpfs("/bar:size=999"),
		ctr.WithCmd("/bin/sh", "-c", "mount | grep '/foo' | grep tmpfs && mount | grep '/bar' | grep tmpfs"),
	)

	defer func() {
		err := apiClient.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
		assert.NilError(t, err)
	}()

	inspect, err := apiClient.ContainerInspect(ctx, id)
	assert.NilError(t, err)
	// tmpfs do not currently get added to inspect.Mounts
	// Normally an anonymous volume would, except now tmpfs should prevent that.
	assert.Assert(t, is.Len(inspect.Mounts, 0))

	chWait, chErr := apiClient.ContainerWait(ctx, id, container.WaitConditionNextExit)
	assert.NilError(t, apiClient.ContainerStart(ctx, id, container.StartOptions{}))

	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()

	select {
	case <-timeout.C:
		t.Fatal("timeout waiting for container to exit")
	case status := <-chWait:
		var errMsg string
		if status.Error != nil {
			errMsg = status.Error.Message
		}
		assert.Equal(t, int(status.StatusCode), 0, errMsg)
	case err := <-chErr:
		assert.NilError(t, err)
	}
}

// Test that if the referenced image platform does not match the requested platform on container create that we get an
// error.
func TestCreateDifferentPlatform(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	img, _, err := apiClient.ImageInspectWithRaw(ctx, "busybox:latest")
	assert.NilError(t, err)
	assert.Assert(t, img.Architecture != "")

	t.Run("different os", func(t *testing.T) {
		ctx := testutil.StartSpan(ctx, t)
		p := ocispec.Platform{
			OS:           img.Os + "DifferentOS",
			Architecture: img.Architecture,
			Variant:      img.Variant,
		}
		_, err := apiClient.ContainerCreate(ctx, &container.Config{Image: "busybox:latest"}, &container.HostConfig{}, nil, &p, "")
		assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
	})
	t.Run("different cpu arch", func(t *testing.T) {
		ctx := testutil.StartSpan(ctx, t)
		p := ocispec.Platform{
			OS:           img.Os,
			Architecture: img.Architecture + "DifferentArch",
			Variant:      img.Variant,
		}
		_, err := apiClient.ContainerCreate(ctx, &container.Config{Image: "busybox:latest"}, &container.HostConfig{}, nil, &p, "")
		assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
	})
}

func TestCreateVolumesFromNonExistingContainer(t *testing.T) {
	ctx := setupTest(t)
	cli := testEnv.APIClient()

	_, err := cli.ContainerCreate(
		ctx,
		&container.Config{Image: "busybox"},
		&container.HostConfig{VolumesFrom: []string{"nosuchcontainer"}},
		nil,
		nil,
		"",
	)
	assert.Check(t, errdefs.IsInvalidParameter(err))
}

// Test that we can create a container from an image that is for a different platform even if a platform was not specified
// This is for the regression detailed here: https://github.com/moby/moby/issues/41552
func TestCreatePlatformSpecificImageNoPlatform(t *testing.T) {
	ctx := setupTest(t)

	skip.If(t, testEnv.DaemonInfo.Architecture == "arm", "test only makes sense to run on non-arm systems")
	skip.If(t, testEnv.DaemonInfo.OSType != "linux", "test image is only available on linux")
	cli := testEnv.APIClient()

	_, err := cli.ContainerCreate(
		ctx,
		&container.Config{Image: "arm32v7/hello-world"},
		&container.HostConfig{},
		nil,
		nil,
		"",
	)
	assert.NilError(t, err)
}

func TestCreateInvalidHostConfig(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	testCases := []struct {
		doc         string
		hc          container.HostConfig
		expectedErr string
	}{
		{
			doc:         "invalid IpcMode",
			hc:          container.HostConfig{IpcMode: "invalid"},
			expectedErr: "Error response from daemon: invalid IPC mode: invalid",
		},
		{
			doc:         "invalid PidMode",
			hc:          container.HostConfig{PidMode: "invalid"},
			expectedErr: "Error response from daemon: invalid PID mode: invalid",
		},
		{
			doc:         "invalid PidMode without container ID",
			hc:          container.HostConfig{PidMode: "container"},
			expectedErr: "Error response from daemon: invalid PID mode: container",
		},
		{
			doc:         "invalid UTSMode",
			hc:          container.HostConfig{UTSMode: "invalid"},
			expectedErr: "Error response from daemon: invalid UTS mode: invalid",
		},
		{
			doc:         "invalid Annotations",
			hc:          container.HostConfig{Annotations: map[string]string{"": "a"}},
			expectedErr: "Error response from daemon: invalid Annotations: the empty string is not permitted as an annotation key",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.StartSpan(ctx, t)
			cfg := container.Config{
				Image: "busybox",
			}
			resp, err := apiClient.ContainerCreate(ctx, &cfg, &tc.hc, nil, nil, "")
			assert.Check(t, is.Equal(len(resp.Warnings), 0))
			assert.Check(t, errdefs.IsInvalidParameter(err), "got: %T", err)
			assert.Error(t, err, tc.expectedErr)
		})
	}
}

func TestCreateWithMultipleEndpointSettings(t *testing.T) {
	ctx := setupTest(t)

	testcases := []struct {
		apiVersion  string
		expectedErr string
	}{
		{apiVersion: "1.44"},
		{apiVersion: "1.43", expectedErr: "Container cannot be created with multiple network endpoints"},
	}

	for _, tc := range testcases {
		t.Run("with API v"+tc.apiVersion, func(t *testing.T) {
			apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion(tc.apiVersion))
			assert.NilError(t, err)

			config := container.Config{
				Image: "busybox",
			}
			networkingConfig := network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{
					"net1": {},
					"net2": {},
					"net3": {},
				},
			}
			_, err = apiClient.ContainerCreate(ctx, &config, &container.HostConfig{}, &networkingConfig, nil, "")
			if tc.expectedErr == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.expectedErr)
			}
		})
	}
}

func TestCreateWithCustomMACs(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.44"), "requires API v1.44")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	net.CreateNoError(ctx, t, apiClient, "testnet")

	attachCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	res := ctr.RunAttach(attachCtx, t, apiClient,
		ctr.WithCmd("ip", "-o", "link", "show"),
		ctr.WithNetworkMode("bridge"),
		ctr.WithMacAddress("bridge", "02:32:1c:23:00:04"))

	assert.Equal(t, res.ExitCode, 0)
	assert.Equal(t, res.Stderr.String(), "")

	scanner := bufio.NewScanner(res.Stdout)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		// The expected output is:
		// 1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue qlen 1000\    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
		// 134: eth0@if135: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1400 qdisc noqueue \    link/ether 02:42:ac:11:00:04 brd ff:ff:ff:ff:ff:ff
		if len(fields) < 11 {
			continue
		}

		ifaceName := fields[1]
		if ifaceName[:3] != "eth" {
			continue
		}

		mac := fields[len(fields)-3]
		assert.Equal(t, mac, "02:32:1c:23:00:04")
	}
}

// Tests that when using containerd backed storage the containerd container has the image referenced stored.
func TestContainerdContainerImageInfo(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.46"), "requires API v1.46")

	ctx := setupTest(t)

	apiClient := testEnv.APIClient()
	defer apiClient.Close()

	info, err := apiClient.Info(ctx)
	assert.NilError(t, err)

	skip.If(t, info.Containerd == nil, "requires containerd")

	// Currently a containerd container is only created when the container is started.
	// So start the container and then inspect the containerd container to verify the image info.
	id := ctr.Run(ctx, t, apiClient, func(cfg *ctr.TestContainerConfig) {
		// busybox is the default (as of this writing) used by the test client, but lets be explicit here.
		cfg.Config.Image = "busybox"
	})
	defer apiClient.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})

	client, err := containerd.New(info.Containerd.Address, containerd.WithDefaultNamespace(info.Containerd.Namespaces.Containers))
	assert.NilError(t, err)
	defer client.Close()

	ctr, err := client.ContainerService().Get(ctx, id)
	assert.NilError(t, err)

	if testEnv.UsingSnapshotter() {
		assert.Equal(t, ctr.Image, "docker.io/library/busybox:latest")
	} else {
		// This field is not set when not using contianerd backed storage.
		assert.Equal(t, ctr.Image, "")
	}
}
