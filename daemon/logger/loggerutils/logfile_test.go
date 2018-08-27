package loggerutils

import (
	"bufio"
	"context"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/tailfile"
	"gotest.tools/assert"
)

func TestTailFiles(t *testing.T) {
	s1 := strings.NewReader("Hello.\nMy name is Inigo Montoya.\n")
	s2 := strings.NewReader("I'm serious.\nDon't call me Shirley!\n")
	s3 := strings.NewReader("Roads?\nWhere we're going we don't need roads.\n")

	files := []SizeReaderAt{s1, s2, s3}
	watcher := logger.NewLogWatcher()
	createDecoder := func(r io.Reader) func() (*logger.Message, error) {
		scanner := bufio.NewScanner(r)
		return func() (*logger.Message, error) {
			if !scanner.Scan() {
				return nil, scanner.Err()
			}
			// some comment
			return &logger.Message{Line: scanner.Bytes(), Timestamp: time.Now()}, nil
		}
	}

	tailReader := func(ctx context.Context, r SizeReaderAt, lines int) (io.Reader, int, error) {
		return tailfile.NewTailReader(ctx, r, lines)
	}

	for desc, config := range map[string]logger.ReadConfig{} {
		t.Run(desc, func(t *testing.T) {
			started := make(chan struct{})
			go func() {
				close(started)
				tailFiles(files, watcher, createDecoder, tailReader, config)
			}()
			<-started
		})
	}

	config := logger.ReadConfig{Tail: 2}
	started := make(chan struct{})
	go func() {
		close(started)
		tailFiles(files, watcher, createDecoder, tailReader, config)
	}()
	<-started

	select {
	case <-time.After(60 * time.Second):
		t.Fatal("timeout waiting for tail line")
	case err := <-watcher.Err:
		assert.Assert(t, err)
	case msg := <-watcher.Msg:
		assert.Assert(t, msg != nil)
		assert.Assert(t, string(msg.Line) == "Roads?", string(msg.Line))
	}

	select {
	case <-time.After(60 * time.Second):
		t.Fatal("timeout waiting for tail line")
	case err := <-watcher.Err:
		assert.Assert(t, err)
	case msg := <-watcher.Msg:
		assert.Assert(t, msg != nil)
		assert.Assert(t, string(msg.Line) == "Where we're going we don't need roads.", string(msg.Line))
	}
}

func TestFollowLogsClose(t *testing.T) {
	lw := logger.NewLogWatcher()

	f, err := ioutil.TempFile("", t.Name())
	assert.NilError(t, err)
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	makeDecoder := func(rdr io.Reader) func() (*logger.Message, error) {
		return func() (*logger.Message, error) {
			return &logger.Message{}, nil
		}
	}

	followLogsDone := make(chan struct{})
	var since, until time.Time
	go func() {
		followLogs(f, lw, make(chan interface{}), makeDecoder, since, until)
		close(followLogsDone)
	}()

	select {
	case <-lw.Msg:
	case err := <-lw.Err:
		assert.NilError(t, err)
	case <-followLogsDone:
		t.Fatal("follow logs finished unexpectedly")
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for log message")
	}

	lw.Close()
	select {
	case <-followLogsDone:
	case <-time.After(20 * time.Second):
		t.Fatal("timeout waiting for followLogs() to finish")
	}
}
