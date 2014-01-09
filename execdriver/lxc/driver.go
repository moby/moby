package lxc

import (
	"github.com/dotcloud/docker/execdriver"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
)

const (
	startPath = "lxc-start"
)

type driver struct {
	containerLock map[string]*sync.Mutex
}

func NewDriver() (execdriver.Driver, error) {
	// setup unconfined symlink
}

func (d *driver) Start(c *execdriver.Container) error {
	l := d.getLock(c)
	l.Lock()
	defer l.Unlock()

	running, err := d.running(c)
	if err != nil {
		return err
	}
	if running {
		return nil
	}

	configPath, err := d.generateConfig(c)
	if err != nil {
		return err
	}

	params := []string{
		startPath,
		"-n", c.Name,
		"-f", configPath,
		"--",
		c.InitPath,
	}

	if c.Network != nil {
		params = append(params,
			"-g", c.Network.Gateway,
			"-i", fmt.Sprintf("%s/%d", c.Network.IPAddress, c.Network, IPPrefixLen),
			"-mtu", c.Network.Mtu,
		)
	}

	if c.User != "" {
		params = append(params, "-u", c.User)
	}

	if c.Privileged {
		params = append(params, "-privileged")
	}

	if c.WorkingDir != "" {
		params = append(params, "-w", c.WorkingDir)
	}

	params = append(params, "--", c.Entrypoint)
	params = append(params, c.Args...)

	cmd := exec.Command(params[0], params[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
	cmd.Stdin = c.Stdin

	if err := cmd.Start(); err != nil {
		return err
	}

	// Poll for running
	if err := d.waitForStart(cmd, c); err != nil {
		return err
	}
	return nil
}

func (d *driver) Stop(c *execdriver.Container) error {
	l := d.getLock(c)
	l.Lock()
	defer l.Unlock()

	if err := d.kill(c, 15); err != nil {
		if err := d.kill(c, 9); err != nil {
			return err
		}
	}

	if err := d.wait(c, 10); err != nil {
		return d.kill(c, 9)
	}
	return nil
}

func (d *driver) Wait(c *execdriver.Container, seconds int) error {
	l := d.getLock(c)
	l.Lock()
	defer l.Unlock()

	return d.wait(c, seconds)
}

// If seconds < 0 then wait forever
func (d *driver) wait(c *execdriver.Container, seconds int) error {

}

func (d *driver) kill(c *execdriver.Container, sig int) error {
	return exec.Command("lxc-kill", "-n", c.Name, strconv.Itoa(sig)).Run()
}

func (d *driver) running(c *execdriver.Container) (bool, error) {

}

// Generate the lxc configuration and return the path to the file
func (d *driver) generateConfig(c *execdriver.Container) (string, error) {

}

func (d *driver) waitForStart(cmd *exec.Cmd, c *execdriver.Container) error {

}

func (d *driver) getLock(c *execdriver.Container) *sync.Mutex {
	l, ok := d.containerLock[c.Name]
	if !ok {
		l = &sync.Mutex{}
		d.containerLock[c.Name] = l
	}
	return l
}
