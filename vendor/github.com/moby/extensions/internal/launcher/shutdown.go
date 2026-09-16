package launcher

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/containerd/log"
	"google.golang.org/grpc"
)

const maxOutputRecordSize = 16 * 1024

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
	if err := signalProcess(cmd, shutdownSignal()); err != nil && !errors.Is(err, os.ErrProcessDone) {
		// os.ErrProcessDone means it already exited, which is the stop we
		// wanted; any other Kill error means we failed to stop it, so report it.
		if killErr := killProcess(cmd); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
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
		if err := killProcess(cmd); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		<-done
		return nil
	}
}

// logOutput drains extension output at info level in bounded records.
func logOutput(ctx context.Context, name string, r io.Reader) {
	br := bufio.NewReaderSize(r, maxOutputRecordSize)
	for {
		record, err := br.ReadSlice('\n')
		if len(record) != 0 {
			// ErrBufferFull marks an arbitrary chunk boundary, not the end of a
			// record. Preserve every byte at that boundary.
			if !errors.Is(err, bufio.ErrBufferFull) {
				record = bytes.TrimRight(record, "\r\n")
			}
			log.G(ctx).WithField("extension", name).Info(string(record))
		}
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) {
			return
		}
	}
}
