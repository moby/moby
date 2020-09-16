package loggerutils

import (
	"bufio"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/pubsub"
	"github.com/docker/docker/pkg/tailfile"
	"gotest.tools/assert"
	"gotest.tools/poll"
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

func TestFollowLogsConsumerGone(t *testing.T) {
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

	lw.ConsumerGone()
	select {
	case <-followLogsDone:
	case <-time.After(20 * time.Second):
		t.Fatal("timeout waiting for followLogs() to finish")
	}
}

func TestFollowLogsProducerGone(t *testing.T) {
	lw := logger.NewLogWatcher()

	f, err := ioutil.TempFile("", t.Name())
	assert.NilError(t, err)
	defer os.Remove(f.Name())

	var sent, received, closed int
	makeDecoder := func(rdr io.Reader) func() (*logger.Message, error) {
		return func() (*logger.Message, error) {
			if closed == 1 {
				closed++
				t.Logf("logDecode() closed after sending %d messages\n", sent)
				return nil, io.EOF
			} else if closed > 1 {
				t.Fatal("logDecode() called after closing!")
				return nil, io.EOF
			}
			sent++
			return &logger.Message{}, nil
		}
	}
	var since, until time.Time

	followLogsDone := make(chan struct{})
	go func() {
		followLogs(f, lw, make(chan interface{}), makeDecoder, since, until)
		close(followLogsDone)
	}()

	// read 1 message
	select {
	case <-lw.Msg:
		received++
	case err := <-lw.Err:
		assert.NilError(t, err)
	case <-followLogsDone:
		t.Fatal("followLogs() finished unexpectedly")
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for log message")
	}

	// "stop" the "container"
	closed = 1
	lw.ProducerGone()

	// should receive all the messages sent
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			select {
			case <-lw.Msg:
				received++
				if received == sent {
					return
				}
			case err := <-lw.Err:
				assert.NilError(t, err)
			}
		}
	}()
	select {
	case <-readDone:
	case <-time.After(30 * time.Second):
		t.Fatalf("timeout waiting for log messages to be read (sent: %d, received: %d", sent, received)
	}

	t.Logf("messages sent: %d, received: %d", sent, received)

	// followLogs() should be done by now
	select {
	case <-followLogsDone:
	case <-time.After(30 * time.Second):
		t.Fatal("timeout waiting for followLogs() to finish")
	}

	select {
	case <-lw.WatchConsumerGone():
		t.Fatal("consumer should not have exited")
	default:
	}
}

func TestCheckCapacityAndRotate(t *testing.T) {
	dir, err := ioutil.TempDir("", t.Name())
	assert.NilError(t, err)
	defer os.RemoveAll(dir)

	f, err := ioutil.TempFile(dir, "log")
	assert.NilError(t, err)

	l := &LogFile{
		f:            f,
		capacity:     5,
		maxFiles:     3,
		compress:     true,
		notifyRotate: pubsub.NewPublisher(0, 1),
		perms:        0600,
		marshal: func(msg *logger.Message) ([]byte, error) {
			return msg.Line, nil
		},
	}
	defer l.Close()

	assert.NilError(t, l.WriteLogEntry(&logger.Message{Line: []byte("hello world!")}))
	_, err = os.Stat(f.Name() + ".1")
	assert.Assert(t, os.IsNotExist(err), dirStringer{dir})

	assert.NilError(t, l.WriteLogEntry(&logger.Message{Line: []byte("hello world!")}))
	poll.WaitOn(t, checkFileExists(f.Name()+".1.gz"), poll.WithDelay(time.Millisecond), poll.WithTimeout(30*time.Second))

	assert.NilError(t, l.WriteLogEntry(&logger.Message{Line: []byte("hello world!")}))
	poll.WaitOn(t, checkFileExists(f.Name()+".1.gz"), poll.WithDelay(time.Millisecond), poll.WithTimeout(30*time.Second))
	poll.WaitOn(t, checkFileExists(f.Name()+".2.gz"), poll.WithDelay(time.Millisecond), poll.WithTimeout(30*time.Second))

	// Now let's simulate a failed rotation where the file was able to be closed but something else happened elsewhere
	// down the line.
	// We want to make sure that we can recover in the case that `l.f` was closed while attempting a rotation.
	l.f.Close()
	assert.NilError(t, l.WriteLogEntry(&logger.Message{Line: []byte("hello world!")}))
}

type dirStringer struct {
	d string
}

func (d dirStringer) String() string {
	ls, err := ioutil.ReadDir(d.d)
	if err != nil {
		return ""
	}
	var s strings.Builder
	s.WriteString("\n")

	for _, fi := range ls {
		s.WriteString(fi.Name() + "\n")
	}
	return s.String()
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
			t.Logf("%s", dirStringer{filepath.Dir(name)})
			return poll.Error(err)
		}
	}
}
