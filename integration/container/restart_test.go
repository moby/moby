package container

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	networktypes "github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	testContainer "github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"github.com/moby/moby/v2/pkg/process"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestDaemonRestartKillContainers(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.IsRootless, "rootless mode doesn't support live-restore")

	ctx := testutil.StartSpan(baseContext, t)

	type testCase struct {
		desc          string
		restartPolicy container.RestartPolicy

		xRunning            bool
		xRunningLiveRestore bool
		xStart              bool
		xHealthCheck        bool
	}

	for _, tc := range []testCase{
		{
			desc:                "container without restart policy",
			xRunningLiveRestore: true,
			xStart:              true,
		},
		{
			desc:                "container with restart=always",
			restartPolicy:       container.RestartPolicy{Name: "always"},
			xRunning:            true,
			xRunningLiveRestore: true,
			xStart:              true,
		},
		{
			desc:                "container with restart=always and with healthcheck",
			restartPolicy:       container.RestartPolicy{Name: "always"},
			xRunning:            true,
			xRunningLiveRestore: true,
			xStart:              true,
			xHealthCheck:        true,
		},
		{
			desc:          "container created should not be restarted",
			restartPolicy: container.RestartPolicy{Name: "always"},
		},
	} {
		for _, liveRestoreEnabled := range []bool{false, true} {
			for fnName, stopDaemon := range map[string]func(*testing.T, *daemon.Daemon){
				"kill-daemon": func(t *testing.T, d *daemon.Daemon) {
					err := d.Kill()
					assert.NilError(t, err)
				},
				"stop-daemon": func(t *testing.T, d *daemon.Daemon) {
					d.Stop(t)
				},
			} {
				tc := tc
				liveRestoreEnabled := liveRestoreEnabled
				t.Run(fmt.Sprintf("live-restore=%v/%s/%s", liveRestoreEnabled, tc.desc, fnName), func(t *testing.T) {
					t.Parallel()

					ctx := testutil.StartSpan(ctx, t)

					d := daemon.New(t)
					apiClient := d.NewClientT(t)

					args := []string{"--iptables=false", "--ip6tables=false"}
					if liveRestoreEnabled {
						args = append(args, "--live-restore")
					}

					d.StartWithBusybox(ctx, t, args...)
					defer d.Stop(t)

					config := container.Config{Image: "busybox", Cmd: []string{"top"}}
					hostConfig := container.HostConfig{RestartPolicy: tc.restartPolicy}
					if tc.xHealthCheck {
						config.Healthcheck = &container.HealthConfig{
							Test:          []string{"CMD-SHELL", "! test -f /tmp/unhealthy"},
							StartPeriod:   60 * time.Second,
							StartInterval: 1 * time.Second,
							Interval:      60 * time.Second,
						}
					}
					resp, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
						Config:     &config,
						HostConfig: &hostConfig,
					})
					assert.NilError(t, err)
					defer apiClient.ContainerRemove(ctx, resp.ID, client.ContainerRemoveOptions{Force: true})

					if tc.xStart {
						_, err = apiClient.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{})
						assert.NilError(t, err)
						if tc.xHealthCheck {
							poll.WaitOn(t, pollForHealthStatus(ctx, apiClient, resp.ID, container.Healthy), poll.WithTimeout(30*time.Second))
							testContainer.ExecT(ctx, t, apiClient, resp.ID, []string{"touch", "/tmp/unhealthy"}).AssertSuccess(t)
						}
					}

					stopDaemon(t, d)
					startTime := time.Now()
					d.Start(t, args...)

					expected := tc.xRunning
					if liveRestoreEnabled {
						expected = tc.xRunningLiveRestore
					}

					poll.WaitOn(t, testContainer.RunningStateFlagIs(ctx, apiClient, resp.ID, expected), poll.WithTimeout(30*time.Second))

					if tc.xHealthCheck {
						// We have arranged to have the container's health probes fail until we tell it
						// to become healthy, which gives us the entire StartPeriod (60s) to assert that
						// the container's health state is Starting before we have to worry about racing
						// the health monitor.
						assert.Equal(t, testContainer.Inspect(ctx, t, apiClient, resp.ID).State.Health.Status, container.Starting)
						poll.WaitOn(t, pollForNewHealthCheck(ctx, apiClient, startTime, resp.ID), poll.WithTimeout(30*time.Second))

						testContainer.ExecT(ctx, t, apiClient, resp.ID, []string{"rm", "/tmp/unhealthy"}).AssertSuccess(t)
						poll.WaitOn(t, pollForHealthStatus(ctx, apiClient, resp.ID, container.Healthy), poll.WithTimeout(30*time.Second))
					}
					// TODO(cpuguy83): test pause states... this seems to be rather undefined currently
				})
			}
		}
	}
}

func pollForNewHealthCheck(ctx context.Context, apiClient *client.Client, startTime time.Time, containerID string) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		inspect, err := apiClient.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
		if err != nil {
			return poll.Error(err)
		}
		healthChecksTotal := len(inspect.Container.State.Health.Log)
		if healthChecksTotal > 0 {
			if inspect.Container.State.Health.Log[healthChecksTotal-1].Start.After(startTime) {
				return poll.Success()
			}
		}
		return poll.Continue("waiting for a new container healthcheck")
	}
}

func TestContainerRestartStoppedContainer(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := testContainer.Create(ctx, t, apiClient, testContainer.WithCmd("sh", "-c", "echo foobar && exit 0"))

	waitTimeout := 10 * time.Second
	if testEnv.DaemonInfo.OSType == "windows" {
		waitTimeout = StopContainerWindowsPollTimeout
	}

	waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()
	wait := apiClient.ContainerWait(waitCtx, cID, client.ContainerWaitOptions{Condition: container.WaitConditionNextExit})

	_, err := apiClient.ContainerStart(ctx, cID, client.ContainerStartOptions{})
	assert.NilError(t, err)
	assertContainerExitCode(t, wait, 0, waitTimeout)
	poll.WaitOn(t, logsContains(ctx, apiClient, cID, "foobar\n"), poll.WithTimeout(waitTimeout))

	waitCtx, cancel = context.WithTimeout(ctx, waitTimeout)
	defer cancel()
	wait = apiClient.ContainerWait(waitCtx, cID, client.ContainerWaitOptions{Condition: container.WaitConditionNextExit})

	_, err = apiClient.ContainerRestart(ctx, cID, client.ContainerRestartOptions{})
	assert.NilError(t, err)
	assertContainerExitCode(t, wait, 0, waitTimeout)
	poll.WaitOn(t, logsContains(ctx, apiClient, cID, "foobar\nfoobar\n"), poll.WithTimeout(waitTimeout))
}

func TestContainerRestartWithVolumes(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := testContainer.Run(ctx, t, apiClient, testContainer.WithVolume(dPath("/test")))

	inspect, err := apiClient.ContainerInspect(ctx, cID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(inspect.Container.Mounts, 1))
	mountSource := inspect.Container.Mounts[0].Source

	_, err = apiClient.ContainerRestart(ctx, cID, client.ContainerRestartOptions{})
	assert.NilError(t, err)

	inspect, err = apiClient.ContainerInspect(ctx, cID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(inspect.Container.Mounts, 1))
	assert.Check(t, is.Equal(inspect.Container.Mounts[0].Source, mountSource))
}

func TestContainerRestartPolicyAfterProcessExit(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "test requires daemon on the same host")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows" && testEnv.GitHubActions(),
		`Windows GitHub-hosted runners consistently failed with "DuplicateHandle: Access is denied". See https://github.com/moby/moby/pull/43479`)
	skip.If(t, testEnv.DaemonInfo.OSType == "windows" && testEnv.DaemonInfo.Isolation != "process")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	for _, tc := range []struct {
		name          string
		manualRestart bool
	}{
		{name: "direct-process-exit"},
		{name: "after-manual-restart", manualRestart: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			cID := testContainer.Run(ctx, t, apiClient, testContainer.WithRestartPolicy(container.RestartPolicyAlways))

			if tc.manualRestart {
				_, err := apiClient.ContainerRestart(ctx, cID, client.ContainerRestartOptions{})
				assert.NilError(t, err)
				poll.WaitOn(t, testContainer.IsInState(ctx, apiClient, cID, container.StateRunning), poll.WithTimeout(30*time.Second))
			}

			killContainerProcess(ctx, t, apiClient, cID)
			poll.WaitOn(t, containerRestartCountIs(ctx, apiClient, cID, 1), poll.WithTimeout(30*time.Second))
			poll.WaitOn(t, testContainer.IsInState(ctx, apiClient, cID, container.StateRunning), poll.WithTimeout(30*time.Second))
		})
	}
}

func TestContainerRestartPolicyUserDefinedNetwork(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRemoteDaemon, "test requires daemon on the same host")
	skip.If(t, testEnv.IsUserNamespace)

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	networkName := "restart-policy-network-" + suffix
	firstName := "restart-first-" + suffix
	secondName := "restart-second-" + suffix

	_, err := apiClient.NetworkCreate(ctx, networkName, client.NetworkCreateOptions{Driver: "bridge"})
	assert.NilError(t, err)

	testContainer.Run(ctx, t, apiClient,
		testContainer.WithName(firstName),
		testContainer.WithNetworkMode(networkName),
		testContainer.WithEndpointSettings(networkName, &networktypes.EndpointSettings{Aliases: []string{"foo"}}),
	)

	secondID := testContainer.Run(ctx, t, apiClient,
		testContainer.WithName(secondName),
		testContainer.WithNetworkMode(networkName),
		testContainer.WithRestartPolicy(container.RestartPolicyAlways),
	)

	testContainer.ExecT(ctx, t, apiClient, secondID, []string{"ping", "-c", "1", firstName}).AssertSuccess(t)
	testContainer.ExecT(ctx, t, apiClient, secondID, []string{"ping", "-c", "1", "foo"}).AssertSuccess(t)

	killContainerProcess(ctx, t, apiClient, secondID)
	poll.WaitOn(t, containerRestartCountIs(ctx, apiClient, secondID, 1), poll.WithTimeout(30*time.Second))
	poll.WaitOn(t, testContainer.IsInState(ctx, apiClient, secondID, container.StateRunning), poll.WithTimeout(30*time.Second))

	testContainer.ExecT(ctx, t, apiClient, secondID, []string{"ping", "-c", "1", firstName}).AssertSuccess(t)
	testContainer.ExecT(ctx, t, apiClient, secondID, []string{"ping", "-c", "1", "foo"}).AssertSuccess(t)
}

func killContainerProcess(ctx context.Context, t *testing.T, apiClient client.APIClient, containerID string) {
	t.Helper()

	inspect, err := apiClient.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	assert.NilError(t, process.Kill(inspect.Container.State.Pid))
}

func assertContainerExitCode(t *testing.T, wait client.ContainerWaitResult, expected int64, timeout time.Duration) {
	t.Helper()

	select {
	case err := <-wait.Error:
		assert.NilError(t, err)
	case res := <-wait.Result:
		assert.Check(t, is.Equal(res.StatusCode, expected))
	case <-time.After(timeout):
		t.Fatal("timeout waiting for container exit")
	}
}

func TestContainerRestartPolicyOnFailure(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	waitTimeout := 10 * time.Second
	if testEnv.DaemonInfo.OSType == "windows" {
		waitTimeout = StopContainerWindowsPollTimeout
	}

	t.Run("does-not-restart-on-success", func(t *testing.T) {
		ctx := testutil.StartSpan(ctx, t)

		resp, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
			Config: &container.Config{
				Image: "busybox",
				Cmd:   []string{"true"},
			},
			HostConfig: &container.HostConfig{
				RestartPolicy: container.RestartPolicy{
					Name:              container.RestartPolicyOnFailure,
					MaximumRetryCount: 3,
				},
			},
		})
		assert.NilError(t, err)
		t.Cleanup(func() {
			apiClient.ContainerRemove(ctx, resp.ID, client.ContainerRemoveOptions{Force: true})
		})

		waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
		defer cancel()
		wait := apiClient.ContainerWait(waitCtx, resp.ID, client.ContainerWaitOptions{Condition: container.WaitConditionNextExit})

		_, err = apiClient.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{})
		assert.NilError(t, err)

		select {
		case err := <-wait.Error:
			assert.NilError(t, err)
		case res := <-wait.Result:
			assert.Check(t, is.Equal(int64(0), res.StatusCode))
		case <-time.After(waitTimeout):
			inspect, _ := apiClient.ContainerInspect(ctx, resp.ID, client.ContainerInspectOptions{})
			t.Fatalf("timeout waiting for container exit: status=%q", inspect.Container.State.Status)
		}

		inspect, err := apiClient.ContainerInspect(ctx, resp.ID, client.ContainerInspectOptions{})
		assert.NilError(t, err)
		assert.Check(t, is.Equal(inspect.Container.State.Status, container.StateExited))
		assert.Check(t, is.Equal(inspect.Container.RestartCount, 0))
		assert.Check(t, is.DeepEqual(inspect.Container.HostConfig.RestartPolicy, container.RestartPolicy{
			Name:              container.RestartPolicyOnFailure,
			MaximumRetryCount: 3,
		}))
	})

	t.Run("can-restart-after-retries", func(t *testing.T) {
		ctx := testutil.StartSpan(ctx, t)

		resp, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
			Config: &container.Config{
				Image: "busybox",
				Cmd:   []string{"false"},
			},
			HostConfig: &container.HostConfig{
				RestartPolicy: container.RestartPolicy{
					Name:              container.RestartPolicyOnFailure,
					MaximumRetryCount: 3,
				},
			},
		})
		assert.NilError(t, err)
		t.Cleanup(func() {
			apiClient.ContainerRemove(ctx, resp.ID, client.ContainerRemoveOptions{Force: true})
		})

		_, err = apiClient.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{})
		assert.NilError(t, err)

		poll.WaitOn(t, containerRestartCountIs(ctx, apiClient, resp.ID, 3), poll.WithTimeout(waitTimeout))
		poll.WaitOn(t, testContainer.IsInState(ctx, apiClient, resp.ID, container.StateExited), poll.WithTimeout(waitTimeout))

		_, err = apiClient.ContainerRestart(ctx, resp.ID, client.ContainerRestartOptions{})
		assert.NilError(t, err)
	})
}

func containerRestartCountIs(ctx context.Context, apiClient client.APIClient, containerID string, expected int) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		inspect, err := apiClient.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
		if err != nil {
			return poll.Error(err)
		}
		if inspect.Container.RestartCount == expected {
			return poll.Success()
		}
		return poll.Continue("waiting for restart count %d, current=%d", expected, inspect.Container.RestartCount)
	}
}

// Container started with --rm should be able to be restarted.
// It should be removed only if killed or stopped
func TestContainerWithAutoRemoveCanBeRestarted(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	noWaitTimeout := 0

	for _, tc := range []struct {
		desc  string
		doSth func(ctx context.Context, containerID string) error
	}{
		{
			desc: "kill",
			doSth: func(ctx context.Context, containerID string) error {
				_, err := apiClient.ContainerKill(ctx, containerID, client.ContainerKillOptions{})
				return err
			},
		},
		{
			desc: "stop",
			doSth: func(ctx context.Context, containerID string) error {
				_, err := apiClient.ContainerStop(ctx, containerID, client.ContainerStopOptions{Timeout: &noWaitTimeout})
				return err
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			testutil.StartSpan(ctx, t)
			cID := testContainer.Run(ctx, t, apiClient,
				testContainer.WithName("autoremove-restart-and-"+tc.desc),
				testContainer.WithAutoRemove,
			)
			defer func() {
				_, err := apiClient.ContainerRemove(ctx, cID, client.ContainerRemoveOptions{Force: true})
				if t.Failed() && err != nil {
					t.Logf("Cleaning up test container failed with error: %v", err)
				}
			}()

			_, err := apiClient.ContainerRestart(ctx, cID, client.ContainerRestartOptions{
				Timeout: &noWaitTimeout,
			})
			assert.NilError(t, err)

			inspect, err := apiClient.ContainerInspect(ctx, cID, client.ContainerInspectOptions{})
			assert.NilError(t, err)
			assert.Assert(t, inspect.Container.State.Status != container.StateRemoving, "Container should not be removing yet")

			poll.WaitOn(t, testContainer.IsInState(ctx, apiClient, cID, container.StateRunning))

			err = tc.doSth(ctx, cID)
			assert.NilError(t, err)

			poll.WaitOn(t, testContainer.IsRemoved(ctx, apiClient, cID))
		})
	}
}

// TestContainerRestartWithCancelledRequest verifies that cancelling a restart
// request does not cancel the restart operation, and still starts the container
// after it was stopped.
//
// Regression test for https://github.com/moby/moby/discussions/46682
func TestContainerRestartWithCancelledRequest(t *testing.T) {
	ctx := setupTest(t)

	testutil.StartSpan(ctx, t)

	// The test relies on "trap" to ignore SIGTERM so that the stop takes
	// the full stopTimeout, giving the client time to cancel the request.
	// On Windows, busybox-w32 doesn't support signal trapping (see
	// https://github.com/rmyorston/busybox-w32/issues/303) so the
	// container may exit immediately on SIGTERM, making the test
	// scenario impossible to set up reliably.
	// Allow multiple attempts on Windows so the test can pass when the timing
	// happens to work out.
	if runtime.GOOS == "windows" {
		for retry := range 10 {
			success := true
			fail := func(t *testing.T) {
				success = false
			}
			t.Run(strconv.Itoa(retry), func(t *testing.T) {
				testContainerRestartWithCancelledRequest(ctx, t, fail)
			})
			if success {
				return
			}
		}
		return
	}

	testContainerRestartWithCancelledRequest(ctx, t, func(t *testing.T) {
		t.Fatal("timeout waiting for restart event")
	})
}

func testContainerRestartWithCancelledRequest(ctx context.Context, t *testing.T, fail func(t *testing.T)) {
	apiClient := testEnv.APIClient()
	// Create a container that ignores SIGTERM and doesn't stop immediately,
	// giving us time to cancel the request.
	//
	// Restarting a container is "stop" (and, if needed, "kill"), then "start"
	// the container. We're trying to create the scenario where the "stop" is
	// handled, but the request was cancelled and therefore the "start" not
	// taking place.
	cID := testContainer.Run(ctx, t, apiClient, testContainer.WithCmd("sh", "-c", "trap 'echo received TERM' TERM; echo ready; while true; do usleep 10; done"))
	defer func() {
		_, err := apiClient.ContainerRemove(ctx, cID, client.ContainerRemoveOptions{Force: true})
		if t.Failed() && err != nil {
			t.Logf("Cleaning up test container failed with error: %v", err)
		}
	}()
	poll.WaitOn(t, logsContains(ctx, apiClient, cID, "ready"))

	// Start listening for events.
	result := apiClient.Events(ctx, client.EventsListOptions{
		Filters: make(client.Filters).Add("container", cID).Add("event", string(events.ActionRestart)),
	})
	messages := result.Messages
	errs := result.Err

	// Make restart request, but cancel the request before the container
	// is (forcibly) killed.
	ctx2, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	stopTimeout := 1
	_, err := apiClient.ContainerRestart(ctx2, cID, client.ContainerRestartOptions{
		Timeout: &stopTimeout,
	})
	assert.Check(t, is.ErrorIs(err, context.DeadlineExceeded))
	cancel()

	// Validate that the restart event occurred, which is emitted
	// after the restart (stop (kill) start) finished.
	//
	// Note that we cannot use RestartCount for this, as that's only
	// used for restart-policies.
	restartTimeout := 10 * time.Second
	select {
	case m := <-messages:
		assert.Check(t, is.Equal(m.Actor.ID, cID))
		assert.Check(t, is.Equal(m.Action, events.ActionRestart))
	case err := <-errs:
		assert.NilError(t, err)
	case <-time.After(restartTimeout):
		fail(t)
		return
	}

	// Container should be restarted (running).
	inspect, err := apiClient.ContainerInspect(ctx, cID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(inspect.Container.State.Status, container.StateRunning))
}

// TestContainerAPIRestart tests that the container restart API endpoint
// restarts a running container with a specified timeout.
//
// Migrated from integration-cli: TestContainerAPIRestart (docker_api_containers_test.go)
// Related to issue https://github.com/moby/moby/issues/50159
func TestContainerAPIRestart(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := testContainer.Run(ctx, t, apiClient)
	poll.WaitOn(t, testContainer.IsInState(ctx, apiClient, cID, container.StateRunning))

	timeout := 1
	_, err := apiClient.ContainerRestart(ctx, cID, client.ContainerRestartOptions{
		Timeout: &timeout,
	})
	assert.NilError(t, err)

	// After restart, the container should be running and not stuck in "restarting" state.
	poll.WaitOn(t, testContainer.IsInState(ctx, apiClient, cID, container.StateRunning), poll.WithTimeout(15*time.Second))
}

// TestContainerAPIRestartNoTimeoutParam tests that the container restart API
// endpoint works correctly when no timeout parameter is provided (uses server default).
//
// Migrated from integration-cli: TestContainerAPIRestartNotimeoutParam (docker_api_containers_test.go)
// Related to issue https://github.com/moby/moby/issues/50159
func TestContainerAPIRestartNoTimeoutParam(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	cID := testContainer.Run(ctx, t, apiClient)
	poll.WaitOn(t, testContainer.IsInState(ctx, apiClient, cID, container.StateRunning))

	_, err := apiClient.ContainerRestart(ctx, cID, client.ContainerRestartOptions{})
	assert.NilError(t, err)

	// After restart, the container should be running and not stuck in "restarting" state.
	poll.WaitOn(t, testContainer.IsInState(ctx, apiClient, cID, container.StateRunning), poll.WithTimeout(15*time.Second))
}
