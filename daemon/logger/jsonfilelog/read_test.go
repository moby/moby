package jsonfilelog // import "github.com/docker/docker/daemon/logger/jsonfilelog"

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/docker/docker/daemon/logger"
	"gotest.tools/assert"
	"gotest.tools/fs"
)

func BenchmarkJSONFileLoggerReadLogs(b *testing.B) {
	tmp := fs.NewDir(b, "bench-jsonfilelog")
	defer tmp.Remove()

	jsonlogger, err := New(logger.Info{
		ContainerID: "a7317399f3f857173c6179d44823594f8294678dea9999662e5c625b5a1c7657",
		LogPath:     tmp.Join("container.log"),
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

	msg := &logger.Message{
		Line:      []byte("Line that thinks that it is log line from docker\n"),
		Source:    "stderr",
		Timestamp: time.Now().UTC(),
	}

	buf := bytes.NewBuffer(nil)
	assert.NilError(b, marshalMessage(msg, nil, buf))
	b.SetBytes(int64(buf.Len()))

	b.ResetTimer()

	chError := make(chan error, b.N+1)
	go func() {
		for i := 0; i < b.N; i++ {
			chError <- jsonlogger.Log(msg)
		}
		chError <- jsonlogger.Close()
	}()

	lw := jsonlogger.(*JSONFileLogger).ReadLogs(logger.ReadConfig{Follow: true})
	for {
		select {
		case <-lw.Msg:
		case <-lw.WatchProducerGone():
			return
		case err := <-chError:
			if err != nil {
				b.Fatal(err)
			}
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

	decode := decodeFunc(buf)
	msg, err := decode()
	assert.NilError(t, err)
	assert.Assert(t, string(msg.Line) == "hello 1\n", string(msg.Line))

	msg, err = decode()
	assert.NilError(t, err)
	assert.Assert(t, string(msg.Line) == "hello 2\n")

	msg, err = decode()
	assert.NilError(t, err)
	assert.Assert(t, string(msg.Line) == "hello 3\n")

	_, err = decode()
	assert.Assert(t, err == io.EOF)
}
