//go:build race

package loggerutils

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/moby/moby/v2/daemon/logger"
	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/moby/moby/v2/pkg/tailfile"
	"golang.org/x/sync/errgroup"
	"gotest.tools/v3/assert"
)

func TestConcurrentLogging(t *testing.T) {
	const (
		containers = 5
		loggers    = 3  // loggers per container
		messages   = 50 // messages per logger

		capacity = 256
		maxFiles = 3
		compress = true
	)
	getTailReader := func(ctx context.Context, r SizeReaderAt, lines int) (SizeReaderAt, int, error) {
		return tailfile.NewTailReader(ctx, r, lines)
	}
	createDecoder := func(io.Reader) Decoder {
		return dummyDecoder{}
	}
	marshal := func(msg *logger.Message) []byte {
		return []byte(fmt.Sprintf(
			"Line=%q Source=%q Timestamp=%v Attrs=%v PLogMetaData=%#v Err=%v",
			msg.Line, msg.Source, msg.Timestamp, msg.Attrs, msg.PLogMetaData, msg.Err,
		))
	}
	g, ctx := errgroup.WithContext(context.Background())
	for ct := 0; ct < containers; ct++ {
		ct := ct
		dir := t.TempDir()
		g.Go(func() (retErr error) {
			logfile, err := NewLogFile(filepath.Join(dir, "log.log"), capacity, maxFiles, compress, createDecoder, 0o644, getTailReader)
			if err != nil {
				return err
			}
			defer func() {
				if err := logfile.Close(); err != nil && retErr == nil {
					retErr = err
				}
			}()
			lg, ctx := errgroup.WithContext(ctx)
			for ln := 0; ln < loggers; ln++ {
				ln := ln
				lg.Go(func() error {
					for m := 0; m < messages; m++ {
						select {
						case <-ctx.Done():
							return ctx.Err()
						default:
						}
						timestamp := time.Now()
						msg := logger.NewMessage()
						msg.Line = append(msg.Line, fmt.Sprintf("container=%v logger=%v msg=%v", ct, ln, m)...)
						msg.Source = "stdout"
						msg.Timestamp = timestamp
						msg.Attrs = append(msg.Attrs, backend.LogAttr{Key: "foo", Value: "bar"})
						msg.PLogMetaData = &backend.PartialLogMetaData{ID: fmt.Sprintf("%v %v %v", ct, ln, m), Ordinal: 1, Last: true}
						marshalled := marshal(msg)
						logger.PutMessage(msg)
						if err := logfile.WriteLogEntry(timestamp, marshalled); err != nil {
							return err
						}
					}
					return nil
				})
			}
			return lg.Wait()
		})
	}
	assert.NilError(t, g.Wait())
}
