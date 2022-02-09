package local

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/plugins/logdriver"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggertest"
	protoio "github.com/gogo/protobuf/io"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestWriteLog(t *testing.T) {
	t.Parallel()

	dir, err := os.MkdirTemp("", t.Name())
	assert.NilError(t, err)
	defer os.RemoveAll(dir)

	logPath := filepath.Join(dir, "test.log")

	l, err := New(logger.Info{LogPath: logPath})
	assert.NilError(t, err)
	defer l.Close()

	m1 := logger.Message{Source: "stdout", Timestamp: time.Now().Add(-1 * 30 * time.Minute), Line: []byte("message 1")}
	m2 := logger.Message{Source: "stdout", Timestamp: time.Now().Add(-1 * 20 * time.Minute), Line: []byte("message 2"), PLogMetaData: &backend.PartialLogMetaData{Last: true, ID: "0001", Ordinal: 1}}
	m3 := logger.Message{Source: "stderr", Timestamp: time.Now().Add(-1 * 10 * time.Minute), Line: []byte("message 3")}

	// copy the log message because the underying log writer resets the log message and returns it to a buffer pool
	err = l.Log(copyLogMessage(&m1))
	assert.NilError(t, err)
	err = l.Log(copyLogMessage(&m2))
	assert.NilError(t, err)
	err = l.Log(copyLogMessage(&m3))
	assert.NilError(t, err)

	f, err := os.Open(logPath)
	assert.NilError(t, err)
	defer f.Close()
	dec := protoio.NewUint32DelimitedReader(f, binary.BigEndian, 1e6)

	var (
		proto     logdriver.LogEntry
		testProto logdriver.LogEntry
		partial   logdriver.PartialLogEntryMetadata
	)

	lenBuf := make([]byte, encodeBinaryLen)
	seekMsgLen := func() {
		io.ReadFull(f, lenBuf)
	}

	err = dec.ReadMsg(&proto)
	assert.NilError(t, err)
	messageToProto(&m1, &testProto, &partial)
	assert.Check(t, is.DeepEqual(testProto, proto), "expected:\n%+v\ngot:\n%+v", testProto, proto)
	seekMsgLen()

	err = dec.ReadMsg(&proto)
	assert.NilError(t, err)
	messageToProto(&m2, &testProto, &partial)
	assert.Check(t, is.DeepEqual(testProto, proto))
	seekMsgLen()

	err = dec.ReadMsg(&proto)
	assert.NilError(t, err)
	messageToProto(&m3, &testProto, &partial)
	assert.Check(t, is.DeepEqual(testProto, proto), "expected:\n%+v\ngot:\n%+v", testProto, proto)
}

func TestReadLog(t *testing.T) {
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

func BenchmarkLogWrite(b *testing.B) {
	f, err := os.CreateTemp("", b.Name())
	assert.Assert(b, err)
	defer os.Remove(f.Name())
	f.Close()

	local, err := New(logger.Info{LogPath: f.Name()})
	assert.Assert(b, err)
	defer local.Close()

	t := time.Now().UTC()
	for _, data := range [][]byte{
		[]byte(""),
		[]byte("a short string"),
		bytes.Repeat([]byte("a long string"), 100),
		bytes.Repeat([]byte("a really long string"), 10000),
	} {
		b.Run(fmt.Sprintf("%d", len(data)), func(b *testing.B) {
			entry := &logdriver.LogEntry{Line: data, Source: "stdout", TimeNano: t.UnixNano()}
			b.SetBytes(int64(entry.Size() + encodeBinaryLen + encodeBinaryLen))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				msg := logger.NewMessage()
				msg.Line = data
				msg.Timestamp = t
				msg.Source = "stdout"
				if err := local.Log(msg); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func copyLogMessage(src *logger.Message) *logger.Message {
	dst := logger.NewMessage()
	dst.Source = src.Source
	dst.Timestamp = src.Timestamp
	dst.Attrs = src.Attrs
	dst.Err = src.Err
	dst.Line = append(dst.Line, src.Line...)
	if src.PLogMetaData != nil {
		lmd := *src.PLogMetaData
		dst.PLogMetaData = &lmd
	}
	return dst
}
