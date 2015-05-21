// +build windows

/*
 This is the Windows driver for containers.

 TODO Windows: It is currently a placeholder to allow compilation of the
 daemon. Future PRs will have an implementation of this driver.
*/

package windows

import (
	"fmt"

	"github.com/docker/docker/daemon/execdriver"
)

const (
	DriverName = "Windows"
	Version    = "Placeholder"
)

type activeContainer struct {
	command *execdriver.Command
}

type driver struct {
	root     string
	initPath string
}

type info struct {
	ID     string
	driver *driver
}

func NewDriver(root, initPath string) (*driver, error) {
	return &driver{
		root:     root,
		initPath: initPath,
	}, nil
}

func (d *driver) Run(c *execdriver.Command, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (execdriver.ExitStatus, error) {
	return execdriver.ExitStatus{ExitCode: 0}, nil
}

func (d *driver) Terminate(p *execdriver.Command) error {
	return nil
}

func (d *driver) Kill(p *execdriver.Command, sig int) error {
	return nil
}

func kill(ID string, PID int) error {
	return nil
}

func (d *driver) Pause(c *execdriver.Command) error {
	return fmt.Errorf("Windows: Containers cannot be paused")
}

func (d *driver) Unpause(c *execdriver.Command) error {
	return fmt.Errorf("Windows: Containers cannot be paused")
}

func (i *info) IsRunning() bool {
	return false
}

func (d *driver) Info(id string) execdriver.Info {
	return &info{
		ID:     id,
		driver: d,
	}
}

func (d *driver) Name() string {
	return fmt.Sprintf("%s Date %s", DriverName, Version)
}

func (d *driver) GetPidsForContainer(id string) ([]int, error) {
	return nil, fmt.Errorf("GetPidsForContainer: GetPidsForContainer() not implemented")
}

func (d *driver) Clean(id string) error {
	return nil
}

func (d *driver) Stats(id string) (*execdriver.ResourceStats, error) {
	return nil, fmt.Errorf("Windows: Stats not implemented")
}

func (d *driver) Exec(c *execdriver.Command, processConfig *execdriver.ProcessConfig, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (int, error) {
	return 0, nil
}
