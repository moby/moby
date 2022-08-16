package loggerutils // import "github.com/docker/docker/daemon/logger/loggerutils"

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/tailfile"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
)

type testDecoder struct {
	rdr        io.Reader
	scanner    *bufio.Scanner
	resetCount int
}

func (d *testDecoder) Decode() (*logger.Message, error) {
	if d.scanner == nil {
		d.scanner = bufio.NewScanner(d.rdr)
	}
	if !d.scanner.Scan() {
		return nil, d.scanner.Err()
	}
	// some comment
	return &logger.Message{Line: d.scanner.Bytes(), Timestamp: time.Now()}, nil
}

func (d *testDecoder) Reset(rdr io.Reader) {
	d.rdr = rdr
	d.scanner = bufio.NewScanner(rdr)
	d.resetCount++
}

func (d *testDecoder) Close() {
	d.rdr = nil
	d.scanner = nil
}

func TestTailFiles(t *testing.T) {
	s1 := strings.NewReader("Hello.\nMy name is Inigo Montoya.\n")
	s2 := strings.NewReader("I'm serious.\nDon't call me Shirley!\n")
	s3 := strings.NewReader("Roads?\nWhere we're going we don't need roads.\n")

	files := []SizeReaderAt{s1, s2, s3}
	watcher := logger.NewLogWatcher()
	defer watcher.ConsumerGone()

	tailReader := func(ctx context.Context, r SizeReaderAt, lines int) (io.Reader, int, error) {
		return tailfile.NewTailReader(ctx, r, lines)
	}
	dec := &testDecoder{}

	for desc, config := range map[string]logger.ReadConfig{} {
		t.Run(desc, func(t *testing.T) {
			started := make(chan struct{})
			fwd := newForwarder(config)
			go func() {
				close(started)
				tailFiles(files, watcher, dec, tailReader, config.Tail, fwd)
			}()
			<-started
		})
	}

	config := logger.ReadConfig{Tail: 2}
	fwd := newForwarder(config)
	started := make(chan struct{})
	go func() {
		close(started)
		tailFiles(files, watcher, dec, tailReader, config.Tail, fwd)
	}()
	<-started

	select {
	case <-time.After(60 * time.Second):
		t.Fatal("timeout waiting for tail line")
	case err := <-watcher.Err:
		assert.NilError(t, err)
	case msg := <-watcher.Msg:
		assert.Assert(t, msg != nil)
		assert.Assert(t, string(msg.Line) == "Roads?", string(msg.Line))
	}

	select {
	case <-time.After(60 * time.Second):
		t.Fatal("timeout waiting for tail line")
	case err := <-watcher.Err:
		assert.NilError(t, err)
	case msg := <-watcher.Msg:
		assert.Assert(t, msg != nil)
		assert.Assert(t, string(msg.Line) == "Where we're going we don't need roads.", string(msg.Line))
	}
}

type dummyDecoder struct{}

func (dummyDecoder) Decode() (*logger.Message, error) {
	return &logger.Message{}, nil
}

func (dummyDecoder) Close()          {}
func (dummyDecoder) Reset(io.Reader) {}

func TestCheckCapacityAndRotate(t *testing.T) {
	dir := t.TempDir()

	logPath := filepath.Join(dir, "log")
	getTailReader := func(ctx context.Context, r SizeReaderAt, lines int) (io.Reader, int, error) {
		return tailfile.NewTailReader(ctx, r, lines)
	}
	createDecoder := func(io.Reader) Decoder {
		return dummyDecoder{}
	}
	l, err := NewLogFile(
		logPath,
		5,    // capacity
		3,    // maxFiles
		true, // compress
		createDecoder,
		0600, // perms
		getTailReader,
	)
	assert.NilError(t, err)
	defer l.Close()

	ls := dirStringer{dir}

	timestamp := time.Time{}

	assert.NilError(t, l.WriteLogEntry(timestamp, []byte("hello world!")))
	_, err = os.Stat(logPath + ".1")
	assert.Assert(t, os.IsNotExist(err), ls)

	assert.NilError(t, l.WriteLogEntry(timestamp, []byte("hello world!")))
	poll.WaitOn(t, checkFileExists(logPath+".1.gz"), poll.WithDelay(time.Millisecond), poll.WithTimeout(30*time.Second))

	assert.NilError(t, l.WriteLogEntry(timestamp, []byte("hello world!")))
	poll.WaitOn(t, checkFileExists(logPath+".1.gz"), poll.WithDelay(time.Millisecond), poll.WithTimeout(30*time.Second))
	poll.WaitOn(t, checkFileExists(logPath+".2.gz"), poll.WithDelay(time.Millisecond), poll.WithTimeout(30*time.Second))

	t.Run("closed log file", func(t *testing.T) {
		// Now let's simulate a failed rotation where the file was able to be closed but something else happened elsewhere
		// down the line.
		// We want to make sure that we can recover in the case that `l.f` was closed while attempting a rotation.
		l.f.Close()
		assert.NilError(t, l.WriteLogEntry(timestamp, []byte("hello world!")))
		assert.NilError(t, os.Remove(logPath+".2.gz"))
	})

	t.Run("with log reader", func(t *testing.T) {
		// Make sure rotate works with an active reader
		lw := l.ReadLogs(logger.ReadConfig{Follow: true, Tail: 1000})
		defer lw.ConsumerGone()

		assert.NilError(t, l.WriteLogEntry(timestamp, []byte("hello world 0!")), ls)
		// make sure the log reader is primed
		waitForMsg(t, lw, 30*time.Second)

		assert.NilError(t, l.WriteLogEntry(timestamp, []byte("hello world 1!")), ls)
		assert.NilError(t, l.WriteLogEntry(timestamp, []byte("hello world 2!")), ls)
		assert.NilError(t, l.WriteLogEntry(timestamp, []byte("hello world 3!")), ls)
		assert.NilError(t, l.WriteLogEntry(timestamp, []byte("hello world 4!")), ls)
		poll.WaitOn(t, checkFileExists(logPath+".2.gz"), poll.WithDelay(time.Millisecond), poll.WithTimeout(30*time.Second))
	})
}

func waitForMsg(t *testing.T, lw *logger.LogWatcher, timeout time.Duration) {
	t.Helper()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case _, ok := <-lw.Msg:
		assert.Assert(t, ok, "log producer gone before log message arrived")
	case err := <-lw.Err:
		assert.NilError(t, err)
	case <-timer.C:
		t.Fatal("timeout waiting for log message")
	}
}

type dirStringer struct {
	d string
}

func (d dirStringer) String() string {
	ls, err := os.ReadDir(d.d)
	if err != nil {
		return ""
	}
	buf := bytes.NewBuffer(nil)
	tw := tabwriter.NewWriter(buf, 1, 8, 1, '\t', 0)
	buf.WriteString("\n")

	btw := bufio.NewWriter(tw)

	for _, entry := range ls {
		fi, err := entry.Info()
		if err != nil {
			return ""
		}

		btw.WriteString(fmt.Sprintf("%s\t%s\t%dB\t%s\n", fi.Name(), fi.Mode(), fi.Size(), fi.ModTime()))
	}
	btw.Flush()
	tw.Flush()
	return buf.String()
}

func checkFileExists(name string) poll.Check {
	return func(t poll.LogT) poll.Result {
		_, err := os.Stat(name)
		switch {
		case err == nil:
			return poll.Success()
		case os.IsNotExist(err):
			return poll.Continue("waiting for %s to exist", name)
		default:
			t.Logf("waiting for %s: %v: %s", name, err, dirStringer{filepath.Dir(name)})
			return poll.Error(err)
		}
	}
}
