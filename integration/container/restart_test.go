package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	testContainer "github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestDaemonRestartKillContainers(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.IsRootless, "rootless mode doesn't support live-restore")
	type testCase struct {
		desc       string
		config     *container.Config
		hostConfig *container.HostConfig

		xRunning            bool
		xRunningLiveRestore bool
		xStart              bool
		xHealthCheck        bool
	}

	for _, tc := range []testCase{
		{
			desc:                "container without restart policy",
			config:              &container.Config{Image: "busybox", Cmd: []string{"top"}},
			xRunningLiveRestore: true,
			xStart:              true,
		},
		{
			desc:                "container with restart=always",
			config:              &container.Config{Image: "busybox", Cmd: []string{"top"}},
			hostConfig:          &container.HostConfig{RestartPolicy: container.RestartPolicy{Name: "always"}},
			xRunning:            true,
			xRunningLiveRestore: true,
			xStart:              true,
		},
		{
			desc: "container with restart=always and with healthcheck",
			config: &container.Config{
				Image: "busybox", Cmd: []string{"top"},
				Healthcheck: &container.HealthConfig{
					Test:     []string{"CMD-SHELL", "sleep 1"},
					Interval: time.Second,
				},
			},
			hostConfig:          &container.HostConfig{RestartPolicy: container.RestartPolicy{Name: "always"}},
			xRunning:            true,
			xRunningLiveRestore: true,
			xStart:              true,
			xHealthCheck:        true,
		},
		{
			desc:       "container created should not be restarted",
			config:     &container.Config{Image: "busybox", Cmd: []string{"top"}},
			hostConfig: &container.HostConfig{RestartPolicy: container.RestartPolicy{Name: "always"}},
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
				stopDaemon := stopDaemon
				t.Run(fmt.Sprintf("live-restore=%v/%s/%s", liveRestoreEnabled, tc.desc, fnName), func(t *testing.T) {
					t.Parallel()

					d := daemon.New(t)
					client := d.NewClientT(t)

					args := []string{"--iptables=false"}
					if liveRestoreEnabled {
						args = append(args, "--live-restore")
					}

					d.StartWithBusybox(t, args...)
					defer d.Stop(t)
					ctx := context.Background()

					resp, err := client.ContainerCreate(ctx, tc.config, tc.hostConfig, nil, nil, "")
					assert.NilError(t, err)
					defer client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})

					if tc.xStart {
						err = client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
						assert.NilError(t, err)
					}

					stopDaemon(t, d)
					d.Start(t, args...)

					expected := tc.xRunning
					if liveRestoreEnabled {
						expected = tc.xRunningLiveRestore
					}

					var running bool
					for i := 0; i < 30; i++ {
						inspect, err := client.ContainerInspect(ctx, resp.ID)
						assert.NilError(t, err)

						running = inspect.State.Running
						if running == expected {
							break
						}
						time.Sleep(2 * time.Second)
					}
					assert.Equal(t, expected, running, "got unexpected running state, expected %v, got: %v", expected, running)

					if tc.xHealthCheck {
						startTime := time.Now()
						ctxPoll, cancel := context.WithTimeout(ctx, 30*time.Second)
						defer cancel()
						poll.WaitOn(t, pollForNewHealthCheck(ctxPoll, client, startTime, resp.ID), poll.WithDelay(100*time.Millisecond))
					}
					// TODO(cpuguy83): test pause states... this seems to be rather undefined currently
				})
			}
		}
	}
}

func pollForNewHealthCheck(ctx context.Context, client *client.Client, startTime time.Time, containerID string) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		inspect, err := client.ContainerInspect(ctx, containerID)
		if err != nil {
			return poll.Error(err)
		}
		healthChecksTotal := len(inspect.State.Health.Log)
		if healthChecksTotal > 0 {
			if inspect.State.Health.Log[healthChecksTotal-1].Start.After(startTime) {
				return poll.Success()
			}
		}
		return poll.Continue("waiting for a new container healthcheck")
	}
}

// Container started with --rm should be able to be restarted.
// It should be removed only if killed or stopped
func TestContainerWithAutoRemoveCanBeRestarted(t *testing.T) {
	defer setupTest(t)()
	cli := testEnv.APIClient()
	ctx := context.Background()

	noWaitTimeout := 0

	for _, tc := range []struct {
		desc  string
		doSth func(ctx context.Context, containerID string) error
	}{
		{
			desc: "kill",
			doSth: func(ctx context.Context, containerID string) error {
				return cli.ContainerKill(ctx, containerID, "SIGKILL")
			},
		},
		{
			desc: "stop",
			doSth: func(ctx context.Context, containerID string) error {
				return cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &noWaitTimeout})
			},
		},
	} {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			cID := testContainer.Run(ctx, t, cli,
				testContainer.WithName("autoremove-restart-and-"+tc.desc),
				testContainer.WithAutoRemove,
			)
			defer func() {
				err := cli.ContainerRemove(ctx, cID, types.ContainerRemoveOptions{Force: true})
				if t.Failed() && err != nil {
					t.Logf("Cleaning up test container failed with error: %v", err)
				}
			}()

			err := cli.ContainerRestart(ctx, cID, container.StopOptions{Timeout: &noWaitTimeout})
			assert.NilError(t, err)

			inspect, err := cli.ContainerInspect(ctx, cID)
			assert.NilError(t, err)
			assert.Assert(t, inspect.State.Status != "removing", "Container should not be removing yet")

			poll.WaitOn(t, testContainer.IsInState(ctx, cli, cID, "running"))

			err = tc.doSth(ctx, cID)
			assert.NilError(t, err)

			poll.WaitOn(t, testContainer.IsRemoved(ctx, cli, cID))
		})
	}
}
