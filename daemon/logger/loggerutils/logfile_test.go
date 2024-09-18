package loggerutils // import "github.com/docker/docker/daemon/logger/loggerutils"

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/tailfile"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
)

type testDecoder struct {
	scanner    *bufio.Scanner
	resetCount int
}

func (d *testDecoder) Decode() (retMsg *logger.Message, retErr error) {
	if !d.scanner.Scan() {
		err := d.scanner.Err()
		if err == nil {
			err = io.EOF
		}
		return nil, err
	}

	// some comment
	return &logger.Message{Line: d.scanner.Bytes(), Timestamp: time.Now()}, nil
}

func (d *testDecoder) Reset(rdr io.Reader) {
	d.scanner = bufio.NewScanner(rdr)
	d.resetCount++
}

func (d *testDecoder) Close() {
	d.scanner = nil
}

// tetsJSONStreamDecoder is used as an easy way to test how [SyntaxError]s are
// handled in the log reader.
type testJSONStreamDecoder struct {
	dec *json.Decoder
}

func (d *testJSONStreamDecoder) Decode() (*logger.Message, error) {
	var m logger.Message
	if err := d.dec.Decode(&m); err != nil {
		return nil, err
	}

	return &m, nil
}

func (d *testJSONStreamDecoder) Reset(rdr io.Reader) {
	d.dec = json.NewDecoder(rdr)
}

func (d *testJSONStreamDecoder) Close() {
	d.dec = nil
}

func TestTailFiles(t *testing.T) {
	s1 := strings.NewReader("Hello.\nMy name is Inigo Montoya.\n")
	s2 := strings.NewReader("I'm serious.\nDon't call me Shirley!\n")
	s3 := strings.NewReader("Roads?\nWhere we're going we don't need roads.\n")

	makeOpener := func(ls ...SizeReaderAt) []fileOpener {
		out := make([]fileOpener, 0, len(ls))
		for i, rdr := range ls {
			out = append(out, &sizeReaderAtOpener{rdr, strconv.Itoa(i)})
		}
		return out
	}

	files := makeOpener(s1, s2, s3)
	watcher := logger.NewLogWatcher()
	defer watcher.ConsumerGone()

	tailReader := func(ctx context.Context, r SizeReaderAt, lines int) (SizeReaderAt, int, error) {
		return tailfile.NewTailReader(ctx, r, lines)
	}
	dec := &testDecoder{}

	config := logger.ReadConfig{Tail: 2}
	fwd := newForwarder(config)
	started := make(chan struct{})
	go func() {
		close(started)
		tailFiles(context.TODO(), files, watcher, dec, tailReader, config.Tail, fwd)
	}()
	<-started

	waitForMsg(t, watcher, "Roads?", 10*time.Second)
	waitForMsg(t, watcher, "Where we're going we don't need roads.", 10*time.Second)

	t.Run("handle corrupted data", func(t *testing.T) {
		// Here we'll use the test json decoder to test injecting garbage data
		// in the middle of otherwise valid json streams
		// The log reader should be able to skip over that data.

		writeMsg := func(buf *bytes.Buffer, s string) {
			t.Helper()

			msg := &logger.Message{Line: []byte(s)}
			dt, err := json.Marshal(msg)
			assert.NilError(t, err)

			_, err = buf.Write(dt)
			assert.NilError(t, err)
			_, err = buf.WriteString("\n")
			assert.NilError(t, err)
		}

		msg1 := "Hello"
		msg2 := "World!"
		msg3 := "And again!"
		msg4 := "One more time!"
		msg5 := "This is the end!"

		f1 := bytes.NewBuffer(nil)
		writeMsg(f1, msg1)

		_, err := f1.WriteString("some randome garbage")
		assert.NilError(t, err, "error writing garbage to log stream")

		writeMsg(f1, msg2) // This won't be seen due to garbage written above

		f2 := bytes.NewBuffer(nil)
		writeMsg(f2, msg3)

		// Write what looks like the start of a new log message
		_, err = f2.WriteString("{\"Line\": ")
		assert.NilError(t, err)

		writeMsg(f2, msg4) // This won't be seen due to garbage written above

		f3 := bytes.NewBuffer(nil)
		writeMsg(f3, msg5)

		// [bytes.Buffer] is not a SizeReaderAt, so we need to convert it here.
		files := makeOpener(bytes.NewReader(f1.Bytes()), bytes.NewReader(f2.Bytes()), bytes.NewReader(f3.Bytes()))

		// At this point we our log "files" should have 4 log messages in it
		// interspersed with some junk that is invalid json.

		// We need a zero size watcher so that we can tell the decoder to give us
		// a syntax error.
		watcher := logger.NewLogWatcher()

		config := logger.ReadConfig{Tail: 4}
		fwd := newForwarder(config)

		started := make(chan struct{})
		done := make(chan struct{})
		go func() {
			close(started)
			tailFiles(context.TODO(), files, watcher, &testJSONStreamDecoder{}, tailReader, config.Tail, fwd)
			close(done)
		}()

		waitOrTimeout := func(ch <-chan struct{}) {
			t.Helper()
			timer := time.NewTimer(10 * time.Second)
			defer timer.Stop()

			select {
			case <-timer.C:
				t.Fatal("timeout waiting for channel")
			case <-ch:
			}
		}

		waitOrTimeout(started)

		// Note that due to how the json decoder works, we won't see anything in f1
		waitForMsg(t, watcher, msg3, 10*time.Second)
		waitForMsg(t, watcher, msg5, 10*time.Second)

		waitOrTimeout(done)
	})
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
	getTailReader := func(ctx context.Context, r SizeReaderAt, lines int) (SizeReaderAt, int, error) {
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
		0o600, // perms
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
		lw := l.ReadLogs(context.TODO(), logger.ReadConfig{Follow: true, Tail: 1000})
		defer lw.ConsumerGone()

		assert.NilError(t, l.WriteLogEntry(timestamp, []byte("hello world 0!\n")), ls)
		// make sure the log reader is primed
		waitForMsg(t, lw, "", 30*time.Second)

		assert.NilError(t, l.WriteLogEntry(timestamp, []byte("hello world 1!")), ls)
		assert.NilError(t, l.WriteLogEntry(timestamp, []byte("hello world 2!")), ls)
		assert.NilError(t, l.WriteLogEntry(timestamp, []byte("hello world 3!")), ls)
		assert.NilError(t, l.WriteLogEntry(timestamp, []byte("hello world 4!")), ls)
		poll.WaitOn(t, checkFileExists(logPath+".2.gz"), poll.WithDelay(time.Millisecond), poll.WithTimeout(30*time.Second))
	})
}

func waitForMsg(t *testing.T, lw *logger.LogWatcher, expected string, timeout time.Duration) {
	t.Helper()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-lw.Err:
		assert.NilError(t, err)
	case msg, ok := <-lw.Msg:
		assert.Assert(t, ok, "log producer gone before log message arrived")
		assert.Check(t, cmp.Equal(string(msg.Line), expected))
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
