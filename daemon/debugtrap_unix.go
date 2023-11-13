//go:build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"os"
	"os/signal"

	"github.com/containerd/log"
	"golang.org/x/sys/unix"

	"github.com/docker/docker/pkg/stack"
)

func (daemon *Daemon) setupDumpStackTrap(root string) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, unix.SIGUSR1)
	go func() {
		for range c {
			path, err := stack.DumpToFile(root)
			if err != nil {
				log.G(context.TODO()).WithError(err).Error("failed to write goroutines dump")
			} else {
				log.G(context.TODO()).Infof("goroutine stacks written to %s", path)
			}
		}
	}()
}
