// +build linux

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/reexec"
	"github.com/vishvananda/netns"
)

func init() {
	reexec.Register("docker-healthcheck", reexecHealthcheck)
}

func reexecHealthcheck() {
	if len(os.Args) < 4 {
		logrus.Fatal("no endpoint or namespace path or timeout provided: %v", os.Args)
	}
	endpoint := os.Args[1]
	sandboxKey := os.Args[2]
	timeout, err := time.ParseDuration(os.Args[3])
	if err != nil {
		logrus.Fatalf("invalid timeout value: %s", err)
	}

	address := "127.0.0.1"

	// Save the current network namespace
	origns, err := netns.Get()
	if err != nil {
		logrus.Fatalf("unable to obtain current network namespace: %s", err)
	}
	defer origns.Close()

	ns, err := netns.GetFromPath(sandboxKey)
	if err != nil {
		logrus.Fatalf("unable to get network namespace for %s: %s", sandboxKey, err)
	}
	defer ns.Close()

	defer netns.Set(origns)
	netns.Set(ns)

	healthcheckResult, err := httpHealthcheck(address, endpoint, timeout)
	if err != nil {
		logrus.Fatal(err)
	}
	fmt.Fprint(os.Stdout, healthcheckResult.Output)
	os.Exit(healthcheckResult.ExitCode)
}

func (p *httpProbe) run(ctx context.Context, d *Daemon, container *container.Container) (*types.HealthcheckResult, error) {
	httpSlice := strslice.StrSlice(container.Config.Healthcheck.Test)[1:]

	endpoint := "/"
	if len(httpSlice) > 0 {
		endpoint = httpSlice[0]
	}

	cmd := reexec.Command("docker-healthcheck", endpoint, container.NetworkSettings.SandboxKey, container.Config.Healthcheck.Timeout.String())
	output := &limitedBuffer{}
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("healthcheck error on re-exec cmd: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				return &types.HealthcheckResult{
					End:      time.Now(),
					ExitCode: status.ExitStatus(),
					Output:   output.String(),
				}, nil
			}
		}
		return nil, fmt.Errorf("healthcheck re-exec error: %v: output: %s", err, output)
	}

	return &types.HealthcheckResult{
		End:      time.Now(),
		ExitCode: 0,
		Output:   output.String(),
	}, nil
}
