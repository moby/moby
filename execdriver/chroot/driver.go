package chroot

import (
	"github.com/dotcloud/docker/execdriver"
	"os/exec"
	"time"
)

type driver struct {
}

func NewDriver() (execdriver.Driver, error) {
	return &driver{}, nil
}

func (d *driver) String() string {
	return "chroot"
}

func (d *driver) Start(c *execdriver.Process) error {
	params := []string{
		"chroot",
		c.Rootfs,
		"/.dockerinit",
		"-driver",
		d.String(),
	}
	params = append(params, c.Entrypoint)
	params = append(params, c.Arguments...)

	var (
		name = params[0]
		arg  = params[1:]
	)
	aname, err := exec.LookPath(name)
	if err != nil {
		aname = name
	}
	c.Path = aname
	c.Args = append([]string{name}, arg...)

	if err := c.Start(); err != nil {
		return err
	}

	go func() {
		if err := c.Wait(); err != nil {
			c.WaitError = err
		}
		close(c.WaitLock)
	}()

	return nil
}

func (d *driver) Kill(p *execdriver.Process, sig int) error {
	return p.Process.Kill()
}

func (d *driver) Wait(id string, duration time.Duration) error {
	panic("No Implemented")
}

func (d *driver) Version() string {
	return "0.1"
}
