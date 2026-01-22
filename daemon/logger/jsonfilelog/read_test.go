package jsonfilelog

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/moby/moby/v2/daemon/logger"
	"github.com/moby/moby/v2/daemon/logger/loggertest"
	"github.com/moby/moby/v2/daemon/server/backend"
	"gotest.tools/v3/assert"
)

func BenchmarkJSONFileLoggerReadLogs(b *testing.B) {
	tmp := b.TempDir()

	jsonlogger, err := New(logger.Info{
		ContainerID: "a7317399f3f857173c6179d44823594f8294678dea9999662e5c625b5a1c7657",
		LogPath:     filepath.Join(tmp, "container.log"),
		Config: map[string]string{
			"labels": "first,second",
		},
		ContainerLabels: map[string]string{
			"first":  "label_value",
			"second": "label_foo",
		},
	})
	assert.NilError(b, err)
	defer jsonlogger.Close()

	const line = "Line that thinks that it is log line from docker\n"
	ts := time.Date(2007, 1, 2, 3, 4, 5, 6, time.UTC)
	msg := func() *logger.Message {
		m := logger.NewMessage()
		m.Line = append(m.Line, line...)
		m.Source = "stderr"
		m.Timestamp = ts
		return m
	}

	var buf bytes.Buffer
	assert.NilError(b, marshalMessage(msg(), nil, &buf))
	b.SetBytes(int64(buf.Len()))

	b.ResetTimer()

	chError := make(chan error)
	go func() {
		for i := 0; i < b.N; i++ {
			if err := jsonlogger.Log(msg()); err != nil {
				chError <- err
			}
		}
		if err := jsonlogger.Close(); err != nil {
			chError <- err
		}
	}()

	lw := jsonlogger.(*JSONFileLogger).ReadLogs(b.Context(), logger.ReadConfig{Follow: true})
	for {
		select {
		case _, ok := <-lw.Msg:
			if !ok {
				return
			}
		case err := <-chError:
			b.Fatal(err)
		}
	}
}

func TestEncodeDecode(t *testing.T) {
	t.Parallel()

	m1 := &logger.Message{Line: []byte("hello 1"), Timestamp: time.Now(), Source: "stdout"}
	m2 := &logger.Message{Line: []byte("hello 2"), Timestamp: time.Now(), Source: "stdout"}
	m3 := &logger.Message{Line: []byte("hello 3"), Timestamp: time.Now(), Source: "stdout"}

	buf := bytes.NewBuffer(nil)
	assert.Assert(t, marshalMessage(m1, nil, buf))
	assert.Assert(t, marshalMessage(m2, nil, buf))
	assert.Assert(t, marshalMessage(m3, nil, buf))

	dec := decodeFunc(buf)
	defer dec.Close()

	msg, err := dec.Decode()
	assert.NilError(t, err)
	assert.Assert(t, string(msg.Line) == "hello 1\n", string(msg.Line))

	msg, err = dec.Decode()
	assert.NilError(t, err)
	assert.Assert(t, string(msg.Line) == "hello 2\n")

	msg, err = dec.Decode()
	assert.NilError(t, err)
	assert.Assert(t, string(msg.Line) == "hello 3\n")

	_, err = dec.Decode()
	assert.Assert(t, errors.Is(err, io.EOF))
}

func TestWriteReassemblesPartialChunks(t *testing.T) {
	t.Parallel()

	// With write-side reassembly, the logger buffers non-last chunks and only
	// writes a single JSON entry once the last chunk arrives.
	tmp := t.TempDir()

	jsonlogger, err := New(logger.Info{
		ContainerID: "partialtest",
		LogPath:     filepath.Join(tmp, "container.log"),
	})
	assert.NilError(t, err)
	defer jsonlogger.Close()

	md1 := &backend.PartialLogMetaData{ID: "p1", Ordinal: 1, Last: false}
	md2 := &backend.PartialLogMetaData{ID: "p1", Ordinal: 2, Last: true}

	assert.NilError(t, jsonlogger.Log(&logger.Message{Line: []byte("hello "), Timestamp: time.Now(), Source: "stdout", PLogMetaData: md1}))
	assert.NilError(t, jsonlogger.Log(&logger.Message{Line: []byte("world"), Timestamp: time.Now(), Source: "stdout", PLogMetaData: md2}))
	assert.NilError(t, jsonlogger.Close())

	lw := jsonlogger.(*JSONFileLogger).ReadLogs(context.TODO(), logger.ReadConfig{Tail: -1})
	defer lw.ConsumerGone()

	msg, err := readMessage(lw)
	assert.NilError(t, err)
	assert.Equal(t, "hello world\n", string(msg.Line))

	_, err = readMessage(lw)
	assert.Assert(t, errors.Is(err, io.EOF))
}

func readMessage(lw *logger.LogWatcher) (*logger.Message, error) {
	select {
	case msg, ok := <-lw.Msg:
		if !ok {
			select {
			case err := <-lw.Err:
				if err != nil {
					return nil, err
				}
			default:
			}
			return nil, io.EOF
		}
		return msg, nil
	case err, ok := <-lw.Err:
		if ok && err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
}

func TestReadLogs(t *testing.T) {
	t.Parallel()
	r := loggertest.Reader{
		Factory: func(t *testing.T, info logger.Info) func(*testing.T) logger.Logger {
			dir := t.TempDir()
			info.LogPath = filepath.Join(dir, info.ContainerID+".log")
			return func(t *testing.T) logger.Logger {
				l, err := New(info)
				assert.NilError(t, err)
				return l
			}
		},
	}
	t.Run("Tail", r.TestTail)
	t.Run("Follow", r.TestFollow)
}

func TestTailLogsWithRotation(t *testing.T) {
	t.Parallel()
	compress := func(cmprs bool) {
		t.Run(fmt.Sprintf("compress=%v", cmprs), func(t *testing.T) {
			t.Parallel()
			(&loggertest.Reader{
				Factory: func(t *testing.T, info logger.Info) func(*testing.T) logger.Logger {
					info.Config = map[string]string{
						"compress": strconv.FormatBool(cmprs),
						"max-size": "1b",
						"max-file": "10",
					}
					dir := t.TempDir()
					t.Cleanup(func() {
						t.Logf("%s:\n%s", t.Name(), dirStringer{dir})
					})
					info.LogPath = filepath.Join(dir, info.ContainerID+".log")
					return func(t *testing.T) logger.Logger {
						l, err := New(info)
						assert.NilError(t, err)
						return l
					}
				},
			}).TestTail(t)
		})
	}
	compress(true)
	compress(false)
}

func TestFollowLogsWithRotation(t *testing.T) {
	t.Parallel()
	compress := func(cmprs bool) {
		t.Run(fmt.Sprintf("compress=%v", cmprs), func(t *testing.T) {
			t.Parallel()
			(&loggertest.Reader{
				Factory: func(t *testing.T, info logger.Info) func(*testing.T) logger.Logger {
					// The log follower can fall behind and drop logs if there are too many
					// rotations in a short time. If that was to happen, loggertest would fail the
					// test. Configure the logger so that there will be only one rotation with the
					// set of logs that loggertest writes.
					info.Config = map[string]string{
						"compress": strconv.FormatBool(cmprs),
						"max-size": "4096b",
						"max-file": "3",
					}
					dir := t.TempDir()
					t.Cleanup(func() {
						t.Logf("%s:\n%s", t.Name(), dirStringer{dir})
					})
					info.LogPath = filepath.Join(dir, info.ContainerID+".log")
					return func(t *testing.T) logger.Logger {
						l, err := New(info)
						assert.NilError(t, err)
						return l
					}
				},
			}).TestFollow(t)
		})
	}
	compress(true)
	compress(false)
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

		fmt.Fprintf(btw, "%s\t%s\t%dB\t%s\n", fi.Name(), fi.Mode(), fi.Size(), fi.ModTime())
	}
	btw.Flush()
	tw.Flush()
	return buf.String()
}
