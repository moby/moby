// +build !windows

package daemon

import (
	"os"
	"os/signal"
	"syscall"

	stackdump "github.com/docker/docker/pkg/signal"

	"github.com/Sirupsen/logrus"
)

func setupDumpStackTrap(root string) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGUSR1)
	go func() {
		for range c {
			path, err := stackdump.DumpStacks(root)
			if err != nil {
				logrus.WithError(err).Error("failed to write goroutines dump")
				continue
			}
			logrus.Infof("goroutine stacks written to %s", path)
		}
	}()
}
