package daemon // import "github.com/docker/docker/daemon"

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/container"
)

const (
	// Longest healthcheck probe output message to store. Longer messages will be truncated.
	maxOutputLen = 4096

	// Default interval between probe runs (from the end of the first to the start of the second).
	// Also the time before the first probe.
	defaultProbeInterval = 30 * time.Second

	// The maximum length of time a single probe run should take. If the probe takes longer
	// than this, the check is considered to have failed.
	defaultProbeTimeout = 30 * time.Second

	// The time given for the container to start before the health check starts considering
	// the container unstable. Defaults to none.
	defaultStartPeriod = 0 * time.Second

	// Default number of consecutive failures of the health check
	// for the container to be considered unhealthy.
	defaultProbeRetries = 3

	// Maximum number of entries to record
	maxLogEntries = 5
)

const (
	// Exit status codes that can be returned by the probe command.

	exitStatusHealthy = 0 // Container is healthy
)

// probe implementations know how to run a particular type of probe.
type probe interface {
	// Perform one run of the check. Returns the exit code and an optional
	// short diagnostic string.
	run(context.Context, *Daemon, *container.Container) (*types.HealthcheckResult, error)
}

// cmdProbe implements the "CMD" probe type.
type cmdProbe struct {
	// Run the command with the system's default shell instead of execing it directly.
	shell bool
}

// exec the healthcheck command in the container.
// Returns the exit code and probe output (if any)
func (p *cmdProbe) run(ctx context.Context, d *Daemon, cntr *container.Container) (*types.HealthcheckResult, error) {
	startTime := time.Now()
	cmdSlice := strslice.StrSlice(cntr.Config.Healthcheck.Test)[1:]
	if p.shell {
		cmdSlice = append(getShell(cntr), cmdSlice...)
	}
	entrypoint, args := d.getEntrypointAndArgs(strslice.StrSlice{}, cmdSlice)
	execConfig := container.NewExecConfig(cntr)
	execConfig.OpenStdin = false
	execConfig.OpenStdout = true
	execConfig.OpenStderr = true
	execConfig.DetachKeys = []byte{}
	execConfig.Entrypoint = entrypoint
	execConfig.Args = args
	execConfig.Tty = false
	execConfig.Privileged = false
	execConfig.User = cntr.Config.User
	execConfig.WorkingDir = cntr.Config.WorkingDir

	linkedEnv, err := d.setupLinkedContainers(cntr)
	if err != nil {
		return nil, err
	}
	execConfig.Env = container.ReplaceOrAppendEnvValues(cntr.CreateDaemonEnvironment(execConfig.Tty, linkedEnv), execConfig.Env)

	d.registerExecCommand(cntr, execConfig)
	d.LogContainerEventWithAttributes(cntr, events.Action(string(events.ActionExecCreate)+": "+execConfig.Entrypoint+" "+strings.Join(execConfig.Args, " ")), map[string]string{
		"execID": execConfig.ID,
	})

	output := &limitedBuffer{}
	probeCtx, cancelProbe := context.WithCancel(ctx)
	defer cancelProbe()
	execErr := make(chan error, 1)

	options := containertypes.ExecStartOptions{
		Stdout: output,
		Stderr: output,
	}

	go func() { execErr <- d.ContainerExecStart(probeCtx, execConfig.ID, options) }()

	// Starting an exec can take a significant amount of time: on the order
	// of 1s in extreme cases. The time it takes dockerd and containerd to
	// start the exec is time that the probe process is not running, and so
	// should not count towards the health check's timeout. Apply a separate
	// timeout to abort if the exec request is wedged.
	tm := time.NewTimer(30 * time.Second)
	defer tm.Stop()
	select {
	case <-tm.C:
		return nil, fmt.Errorf("timed out starting health check for container %s", cntr.ID)
	case err := <-execErr:
		if err != nil {
			return nil, err
		}
	case <-execConfig.Started:
		healthCheckStartDuration.UpdateSince(startTime)
	}

	if !tm.Stop() {
		<-tm.C
	}
	probeTimeout := timeoutWithDefault(cntr.Config.Healthcheck.Timeout, defaultProbeTimeout)
	tm.Reset(probeTimeout)
	select {
	case <-tm.C:
		cancelProbe()
		log.G(ctx).WithContext(ctx).Debugf("Health check for container %s taking too long", cntr.ID)
		// Wait for probe to exit (it might take some time to call containerd to kill
		// the process and we don't want dying probes to pile up).
		<-execErr

		var msg string
		if out := output.String(); len(out) > 0 {
			msg = fmt.Sprintf("Health check exceeded timeout (%v): %s", probeTimeout, out)
		} else {
			msg = fmt.Sprintf("Health check exceeded timeout (%v)", probeTimeout)
		}
		return &types.HealthcheckResult{
			ExitCode: -1,
			Output:   msg,
			End:      time.Now(),
		}, nil
	case err := <-execErr:
		if err != nil {
			return nil, err
		}
	}

	info, err := d.getExecConfig(execConfig.ID)
	if err != nil {
		return nil, err
	}
	exitCode, err := func() (int, error) {
		info.Lock()
		defer info.Unlock()
		if info.ExitCode == nil {
			return 0, fmt.Errorf("healthcheck for container %s has no exit code", cntr.ID)
		}
		return *info.ExitCode, nil
	}()
	if err != nil {
		return nil, err
	}
	// Note: Go's json package will handle invalid UTF-8 for us
	out := output.String()
	return &types.HealthcheckResult{
		End:      time.Now(),
		ExitCode: exitCode,
		Output:   out,
	}, nil
}

// Update the container's Status.Health struct based on the latest probe's result.
func handleProbeResult(d *Daemon, c *container.Container, result *types.HealthcheckResult, done chan struct{}) {
	c.Lock()
	defer c.Unlock()

	// probe may have been cancelled while waiting on lock. Ignore result then
	select {
	case <-done:
		return
	default:
	}

	retries := c.Config.Healthcheck.Retries
	if retries <= 0 {
		retries = defaultProbeRetries
	}

	h := c.State.Health
	oldStatus := h.Status()

	if len(h.Log) >= maxLogEntries {
		h.Log = append(h.Log[len(h.Log)+1-maxLogEntries:], result)
	} else {
		h.Log = append(h.Log, result)
	}

	if result.ExitCode == exitStatusHealthy {
		h.FailingStreak = 0
		h.SetStatus(types.Healthy)
	} else { // Failure (including invalid exit code)
		shouldIncrementStreak := true

		// If the container is starting (i.e. we never had a successful health check)
		// then we check if we are within the start period of the container in which
		// case we do not increment the failure streak.
		if h.Status() == types.Starting {
			startPeriod := timeoutWithDefault(c.Config.Healthcheck.StartPeriod, defaultStartPeriod)
			timeSinceStart := result.Start.Sub(c.State.StartedAt)

			// If still within the start period, then don't increment failing streak.
			if timeSinceStart < startPeriod {
				shouldIncrementStreak = false
			}
		}

		if shouldIncrementStreak {
			h.FailingStreak++

			if h.FailingStreak >= retries {
				h.SetStatus(types.Unhealthy)
			}
		}
		// Else we're starting or healthy. Stay in that state.
	}

	// replicate Health status changes
	if err := c.CheckpointTo(d.containersReplica); err != nil {
		// queries will be inconsistent until the next probe runs or other state mutations
		// checkpoint the container
		log.G(context.TODO()).Errorf("Error replicating health state for container %s: %v", c.ID, err)
	}

	current := h.Status()
	if oldStatus != current {
		d.LogContainerEvent(c, events.Action(string(events.ActionHealthStatus)+": "+current))
	}
}

// Run the container's monitoring thread until notified via "stop".
// There is never more than one monitor thread running per container at a time.
func monitor(d *Daemon, c *container.Container, stop chan struct{}, probe probe) {
	probeInterval := timeoutWithDefault(c.Config.Healthcheck.Interval, defaultProbeInterval)
	startInterval := timeoutWithDefault(c.Config.Healthcheck.StartInterval, probeInterval)
	startPeriod := timeoutWithDefault(c.Config.Healthcheck.StartPeriod, defaultStartPeriod)

	c.Lock()
	started := c.State.StartedAt
	c.Unlock()

	getInterval := func() time.Duration {
		if time.Since(started) >= startPeriod {
			return probeInterval
		}
		c.Lock()
		status := c.Health.Health.Status
		c.Unlock()

		if status == types.Starting {
			return startInterval
		}
		return probeInterval
	}

	intervalTimer := time.NewTimer(getInterval())
	defer intervalTimer.Stop()

	for {
		select {
		case <-stop:
			log.G(context.TODO()).Debugf("Stop healthcheck monitoring for container %s (received while idle)", c.ID)
			return
		case <-intervalTimer.C:
			log.G(context.TODO()).Debugf("Running health check for container %s ...", c.ID)
			startTime := time.Now()
			ctx, cancelProbe := context.WithCancel(context.Background())
			results := make(chan *types.HealthcheckResult, 1)
			go func() {
				healthChecksCounter.Inc()
				result, err := probe.run(ctx, d, c)
				if err != nil {
					healthChecksFailedCounter.Inc()
					log.G(ctx).Warnf("Health check for container %s error: %v", c.ID, err)
					results <- &types.HealthcheckResult{
						ExitCode: -1,
						Output:   err.Error(),
						Start:    startTime,
						End:      time.Now(),
					}
				} else {
					result.Start = startTime
					log.G(ctx).Debugf("Health check for container %s done (exitCode=%d)", c.ID, result.ExitCode)
					results <- result
				}
				close(results)
			}()
			select {
			case <-stop:
				log.G(ctx).Debugf("Stop healthcheck monitoring for container %s (received while probing)", c.ID)
				cancelProbe()
				// Wait for probe to exit (it might take a while to respond to the TERM
				// signal and we don't want dying probes to pile up).
				<-results
				return
			case result := <-results:
				handleProbeResult(d, c, result, stop)
				cancelProbe()
			}
		}
		intervalTimer.Reset(getInterval())
	}
}

// Get a suitable probe implementation for the container's healthcheck configuration.
// Nil will be returned if no healthcheck was configured or NONE was set.
func getProbe(c *container.Container) probe {
	config := c.Config.Healthcheck
	if config == nil || len(config.Test) == 0 {
		return nil
	}
	switch config.Test[0] {
	case "CMD":
		return &cmdProbe{shell: false}
	case "CMD-SHELL":
		return &cmdProbe{shell: true}
	case "NONE":
		return nil
	default:
		log.G(context.TODO()).Warnf("Unknown healthcheck type '%s' (expected 'CMD') in container %s", config.Test[0], c.ID)
		return nil
	}
}

// Ensure the health-check monitor is running or not, depending on the current
// state of the container.
// Called from monitor.go, with c locked.
func (daemon *Daemon) updateHealthMonitor(c *container.Container) {
	h := c.State.Health
	if h == nil {
		return // No healthcheck configured
	}

	probe := getProbe(c)
	wantRunning := c.Running && !c.Paused && probe != nil
	if wantRunning {
		if stop := h.OpenMonitorChannel(); stop != nil {
			go monitor(daemon, c, stop, probe)
		}
	} else {
		h.CloseMonitorChannel()
	}
}

// Reset the health state for a newly-started, restarted or restored container.
// initHealthMonitor is called from monitor.go and we should never be running
// two instances at once.
// Called with c locked.
func (daemon *Daemon) initHealthMonitor(c *container.Container) {
	// If no healthcheck is setup then don't init the monitor
	if getProbe(c) == nil {
		return
	}

	// This is needed in case we're auto-restarting
	daemon.stopHealthchecks(c)

	if h := c.State.Health; h != nil {
		h.SetStatus(types.Starting)
		h.FailingStreak = 0
	} else {
		h := &container.Health{}
		h.SetStatus(types.Starting)
		c.State.Health = h
	}

	daemon.updateHealthMonitor(c)
}

// Called when the container is being stopped (whether because the health check is
// failing or for any other reason).
func (daemon *Daemon) stopHealthchecks(c *container.Container) {
	h := c.State.Health
	if h != nil {
		h.CloseMonitorChannel()
	}
}

// Buffer up to maxOutputLen bytes. Further data is discarded.
type limitedBuffer struct {
	buf       bytes.Buffer
	mu        sync.Mutex
	truncated bool // indicates that data has been lost
}

// Append to limitedBuffer while there is room.
func (b *limitedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	bufLen := b.buf.Len()
	dataLen := len(data)
	keep := minInt(maxOutputLen-bufLen, dataLen)
	if keep > 0 {
		b.buf.Write(data[:keep])
	}
	if keep < dataLen {
		b.truncated = true
	}
	return dataLen, nil
}

// The contents of the buffer, with "..." appended if it overflowed.
func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	out := b.buf.String()
	if b.truncated {
		out = out + "..."
	}
	return out
}

// If configuredValue is zero, use defaultValue instead.
func timeoutWithDefault(configuredValue time.Duration, defaultValue time.Duration) time.Duration {
	if configuredValue == 0 {
		return defaultValue
	}
	return configuredValue
}

func minInt(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func getShell(cntr *container.Container) []string {
	if len(cntr.Config.Shell) != 0 {
		return cntr.Config.Shell
	}
	if runtime.GOOS != "windows" {
		return []string{"/bin/sh", "-c"}
	}
	if cntr.OS != runtime.GOOS {
		return []string{"/bin/sh", "-c"}
	}
	return []string{"cmd", "/S", "/C"}
}
