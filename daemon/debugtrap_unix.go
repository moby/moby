// +build !windows

package daemon

import (
	"os"
	"os/signal"

	"github.com/Sirupsen/logrus"
	stackdump "github.com/docker/docker/pkg/signal"
	"golang.org/x/sys/unix"
)

func (d *Daemon) setupDumpStackTrap(root string) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, unix.SIGUSR1)
	go func() {
		for range c {
			path, err := stackdump.DumpStacks(root)
			if err != nil {
				logrus.WithError(err).Error("failed to write goroutines dump")
			} else {
				logrus.Infof("goroutine stacks written to %s", path)
			}
		}
	}()
}
