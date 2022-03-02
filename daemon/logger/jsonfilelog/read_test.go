package jsonfilelog // import "github.com/docker/docker/daemon/logger/jsonfilelog"

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggertest"
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

func TestUnexpectedEOF(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	msg1 := &logger.Message{Timestamp: time.Now(), Line: []byte("hello1")}
	msg2 := &logger.Message{Timestamp: time.Now(), Line: []byte("hello2")}

	err := marshalMessage(msg1, json.RawMessage{}, buf)
	assert.NilError(t, err)
	err = marshalMessage(msg2, json.RawMessage{}, buf)
	assert.NilError(t, err)

	r := &readerWithErr{
		err:   io.EOF,
		after: buf.Len() / 4,
		r:     buf,
	}
	dec := &decoder{rdr: r, maxRetry: 1}

	_, err = dec.Decode()
	assert.Error(t, err, io.ErrUnexpectedEOF.Error())
	// again just to check
	_, err = dec.Decode()
	assert.Error(t, err, io.ErrUnexpectedEOF.Error())

	// reset the error
	// from here all reads should succeed until we get EOF on the underlying reader
	r.err = nil

	msg, err := dec.Decode()
	assert.NilError(t, err)
	assert.Equal(t, string(msg1.Line)+"\n", string(msg.Line))

	msg, err = dec.Decode()
	assert.NilError(t, err)
	assert.Equal(t, string(msg2.Line)+"\n", string(msg.Line))

	_, err = dec.Decode()
	assert.Error(t, err, io.EOF.Error())
}

func TestReadLogs(t *testing.T) {
	loggertest.Reader{
		Factory: func(t *testing.T, info logger.Info) func(*testing.T) logger.Logger {
			dir := t.TempDir()
			info.LogPath = filepath.Join(dir, info.ContainerID+".log")
			return func(t *testing.T) logger.Logger {
				l, err := New(info)
				assert.NilError(t, err)
				return l
			}
		},
	}.Do(t)
}

type readerWithErr struct {
	err   error
	after int
	r     io.Reader
	read  int
}

func (r *readerWithErr) Read(p []byte) (int, error) {
	if r.err != nil && r.read > r.after {
		return 0, r.err
	}

	n, err := r.r.Read(p[:1])
	if n > 0 {
		r.read += n
	}
	return n, err
}
