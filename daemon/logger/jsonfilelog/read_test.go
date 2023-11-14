package jsonfilelog // import "github.com/docker/docker/daemon/logger/jsonfilelog"

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"text/tabwriter"
	"time"

	"gotest.tools/v3/assert"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggertest"
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

	lw := jsonlogger.(*JSONFileLogger).ReadLogs(logger.ReadConfig{Follow: true})
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
	assert.Assert(t, err == io.EOF)
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

		btw.WriteString(fmt.Sprintf("%s\t%s\t%dB\t%s\n", fi.Name(), fi.Mode(), fi.Size(), fi.ModTime()))
	}
	btw.Flush()
	tw.Flush()
	return buf.String()
}
