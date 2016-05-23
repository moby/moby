package daemon

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/exec"
	"github.com/docker/engine-api/types/strslice"
)

const (
	// Longest healthcheck probe output message to store. Longer messages will be truncated.
	maxOutputLen = 255

	// Default interval between probe runs (from the end of the first to the start of the second).
	// Also the time before the first probe.
	defaultProbeInterval = 30 * time.Second

	// The maximum length of time a single probe run should take. If the probe takes longer
	// than this, the check is considered to have failed.
	defaultProbeTimeout = 30 * time.Second

	// Probes started within this period do not cause transitions from Starting to Unhealthy.
	// Instead, we remain in Starting in this case (however, we can still move to Unhealthy after
	// a successful probe, even if we're still in the grace period).
	defaultGracePeriod = 30 * time.Second

	// Shut down a container if it becomes Unhealthy.
	defaultExitOnUnhealthy = true
)

// probeResult is used to pass the results of a probe run back to the monitor thread.
type probeResult struct {
	endTime  time.Time
	exitCode int
	output   string
}

// probe implementations know how to run a particular type of probe.
type probe interface {
	// Perform one run of the check. Returns the exit code and an optional
	// short diagnostic string.
	run(context.Context, *Daemon, *container.Container) (*probeResult, error)
}

// cmdProbe implements the "CMD" probe type.
type cmdProbe struct {
	// Run the command with the system's default shell instead of execing it directly.
	shell bool
}

// exec the healthcheck command in the container.
// Returns the exit code and probe output (if any)
func (p *cmdProbe) run(ctx context.Context, d *Daemon, container *container.Container) (*probeResult, error) {
	cmdSlice := strslice.StrSlice(container.Config.Healthcheck.Test)[1:]
	if p.shell {
		if runtime.GOOS != "windows" {
			cmdSlice = append([]string{"/bin/sh", "-c"}, cmdSlice...)
		} else {
			cmdSlice = append([]string{"cmd", "/S", "/C"}, cmdSlice...)
		}
	}
	entrypoint, args := d.getEntrypointAndArgs(strslice.StrSlice{}, cmdSlice)
	execConfig := exec.NewConfig()
	execConfig.OpenStdin = false
	execConfig.OpenStdout = true
	execConfig.OpenStderr = true
	execConfig.ContainerID = container.ID
	execConfig.DetachKeys = []byte{}
	execConfig.Entrypoint = entrypoint
	execConfig.Args = args
	execConfig.Tty = false
	execConfig.Privileged = false
	execConfig.User = container.Config.User

	d.registerExecCommand(container, execConfig)
	d.LogContainerEvent(container, "exec_create: "+execConfig.Entrypoint+" "+strings.Join(execConfig.Args, " "))

	output := &limitedBuffer{}
	err := d.ContainerExecStart(ctx, execConfig.ID, nil, output, output)
	if err != nil {
		return nil, err
	}
	info, err := d.getExecConfig(execConfig.ID)
	if err != nil {
		return nil, err
	}
	if info.ExitCode == nil {
		return nil, fmt.Errorf("Healthcheck has no exit code!")
	}
	// Note: Go's json package will handle invalid UTF-8 for us
	out := output.String()
	return &probeResult{
		endTime:  time.Now(),
		exitCode: *info.ExitCode,
		output:   out,
	}, nil
}

// Update the container's Status.Health struct based on the latest probe's result.
func handleProbeResult(d *Daemon, c *container.Container, result *probeResult) {
	c.Lock()
	defer c.Unlock()

	graceEnd := c.State.StartedAt.Add(timeoutWithDefault(c.Config.Healthcheck.GracePeriod, defaultGracePeriod))
	retries := uint64(c.Config.Healthcheck.Retries)
	if retries == 0 {
		retries = 1 // Default if unset
	}

	h := c.State.Health
	oldStatus := h.Status

	h.LastCheckEnd = result.endTime
	h.LastOutput = result.output
	h.LastExitCode = result.exitCode

	if result.exitCode == 0 {
		// Success
		h.FailingStreak = 0
		h.Status = container.Healthy
	} else if c.State.Health.Status != container.Starting ||
		c.State.Health.LastCheckStart.After(graceEnd) {
		// Failure (excluding ignored failures during grace period)
		h.FailingStreak++
		if c.State.Health.FailingStreak >= retries {
			h.Status = container.Unhealthy
		}
		// Else we're starting or healthy. Stay in that state.
	}
	// Else we failed while starting and within grace period. Remain starting.

	if oldStatus != h.Status {
		d.LogContainerEvent(c, "health_status: "+h.Status)
	}
}

// Run the container's monitoring thread until notified via "stop".
// There is never more than one monitor thread running per container at a time.
func monitor(d *Daemon, c *container.Container, stop chan struct{}, probe probe) {
	probeTimeout := timeoutWithDefault(c.Config.Healthcheck.Timeout, defaultProbeTimeout)
	probeInterval := timeoutWithDefault(c.Config.Healthcheck.Interval, defaultProbeInterval)
	for {
		select {
		case <-stop:
			logrus.Debugf("Stop healthcheck monitoring (received while idle)")
			return
		case <-time.After(probeInterval):
			logrus.Debugf("Running health check...")
			ctx, cancelProbe := context.WithTimeout(context.Background(), probeTimeout)
			c.Lock()
			c.State.Health.LastCheckStart = time.Now()
			c.Unlock()
			results := make(chan *probeResult)
			go func() {
				result, err := probe.run(ctx, d, c)
				if err != nil {
					logrus.Warnf("Health check error: %v", err)
					results <- &probeResult{
						exitCode: -1,
						output:   err.Error(),
						endTime:  time.Now(),
					}
				} else {
					logrus.Debugf("Health check done (exitCode=%d)", result.exitCode)
					results <- result
				}
				close(results)
			}()
			select {
			case <-stop:
				logrus.Debugf("Stop healthcheck monitoring (received while probing)")
				// Stop timeout and kill probe, but don't wait for probe to exit.
				cancelProbe()
				return
			case result := <-results:
				handleProbeResult(d, c, result)
				// Stop timeout
				cancelProbe()
			case <-ctx.Done():
				logrus.Debugf("Health check taking too long")
				handleProbeResult(d, c, &probeResult{
					exitCode: -1,
					output:   fmt.Sprintf("Health check exceeded timeout (%v)", probeTimeout),
					endTime:  time.Now(),
				})
				cancelProbe()
				// Wait for probe to exit (it might take a while to respond to the TERM
				// signal and we don't want dying probes to pile up).
				<-results
			}
		}
	}
}

// Get a suitable probe implementation for the container's healthcheck configuration.
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
	default:
		logrus.Warnf("Unknown healthcheck type '%s' (expected 'CMD')", config.Test[0])
		return nil
	}
}

// Ensure the health-check monitor is running or not, depending on the current
// state of the container.
// Called from monitor.go, with c locked.
func (d *Daemon) updateHealthMonitor(c *container.Container) {
	h := c.State.Health
	if h == nil {
		return // No healthcheck configured
	}

	probe := getProbe(c)
	wantRunning := c.Running && !c.Paused && probe != nil
	if wantRunning {
		if stop := h.OpenMonitorChannel(); stop != nil {
			go monitor(d, c, stop, probe)
		}
	} else {
		h.CloseMonitorChannel()
	}
}

// Reset the health state for a newly-started, restarted or restored container.
// initHealthMonitor is called from monitor.go and we should never be running
// two instances at once.
// Called with c locked.
func (d *Daemon) initHealthMonitor(c *container.Container) {
	if c.Config.Healthcheck == nil {
		return
	}

	// This is needed in case we're auto-restarting
	d.stopHealthchecks(c)

	if c.State.Health == nil {
		h := &container.Health{}
		h.Status = container.Starting
		h.FailingStreak = 0
		c.State.Health = h
	}

	d.updateHealthMonitor(c)
}

// Called when the container is being stopped (whether because the health check is
// failing or for any other reason).
func (d *Daemon) stopHealthchecks(c *container.Container) {
	h := c.State.Health
	if h != nil {
		h.CloseMonitorChannel()
	}
}

// Buffer up to maxOutputLen bytes. Further data is discarded.
type limitedBuffer struct {
	buf       bytes.Buffer
	truncated bool // indicates that data has been lost
}

// Append to limitedBuffer while there is room.
func (b *limitedBuffer) Write(data []byte) (int, error) {
	bufLen := b.buf.Len()
	dataLen := len(data)
	keep := min(maxOutputLen-bufLen, dataLen)
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
	out := b.buf.String()
	if b.truncated {
		out = out + "..."
	}
	return out
}

// If configuredValue is nil, use defaultValue instead.
func timeoutWithDefault(configuredValue *float64, defaultValue time.Duration) time.Duration {
	if configuredValue == nil {
		return defaultValue
	}
	// Careful: time.Duration(configuredValue) * time.Second will round
	// down to a whole number of seconds (probably zero).
	return time.Duration(*configuredValue * float64(time.Second.Nanoseconds()))
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
