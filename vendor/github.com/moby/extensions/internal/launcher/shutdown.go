package launcher

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/containerd/log"
	"google.golang.org/grpc"
)

type processLifetime interface {
	Close() error
}

type processShutdown struct {
	conn     *grpc.ClientConn
	cmd      *exec.Cmd
	wait     <-chan error
	timeout  time.Duration
	lifetime processLifetime
	once     sync.Once
	err      error
}

// Close stops the extension once. The host and broker may both call it during
// failure cleanup, so repeated calls are no-ops.
func (s *processShutdown) Close(ctx context.Context) error {
	s.once.Do(func() {
		s.err = errors.Join(
			s.conn.Close(),
			stopProcess(ctx, s.cmd, s.wait, s.timeout),
			s.lifetime.Close(),
		)
	})
	return s.err
}

// stopErr ignores the exit status of a process that was deliberately stopped.
func stopErr(err error) error {
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		return nil
	}
	return err
}

func stopProcess(ctx context.Context, cmd *exec.Cmd, done <-chan error, timeout time.Duration) error {
	if cmd.Process == nil {
		return nil
	}

	// Ask the extension to stop. If signaling is unsupported, fall back to Kill.
	if err := cmd.Process.Signal(shutdownSignal()); err != nil && !errors.Is(err, os.ErrProcessDone) {
		// os.ErrProcessDone means it already exited, which is the stop we
		// wanted; any other Kill error means we failed to stop it, so report it.
		if killErr := cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			return fmt.Errorf("kill extension after failed signal %v: %w", err, killErr)
		}
		<-done
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	select {
	case err := <-done:
		return stopErr(err)
	case <-shutdownCtx.Done():
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		<-done
		return nil
	}
}

// logOutput drains extension output at info level. A bufio.Reader avoids the
// scanner token limit, which could otherwise block the extension on a full pipe.
func logOutput(ctx context.Context, name string, r io.Reader) {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			log.G(ctx).WithField("extension", name).Info(strings.TrimRight(line, "\r\n"))
		}
		if err != nil {
			return
		}
	}
}
