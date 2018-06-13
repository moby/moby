package logger // import "github.com/docker/docker/daemon/logger"

import (
	"encoding/binary"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/plugins/logdriver"
	protoio "github.com/gogo/protobuf/io"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

// mockLoggingPlugin implements the loggingPlugin interface for testing purposes
// it only supports a single log stream
type mockLoggingPlugin struct {
	io.WriteCloser
	inStream io.Reader
	logs     []*logdriver.LogEntry
	c        *sync.Cond
	err      error
}

func newMockLoggingPlugin() *mockLoggingPlugin {
	r, w := io.Pipe()
	return &mockLoggingPlugin{
		WriteCloser: w,
		inStream:    r,
		logs:        []*logdriver.LogEntry{},
		c:           sync.NewCond(new(sync.Mutex)),
	}
}

func (l *mockLoggingPlugin) StartLogging(file string, info Info) error {
	go func() {
		dec := protoio.NewUint32DelimitedReader(l.inStream, binary.BigEndian, 1e6)
		for {
			var msg logdriver.LogEntry
			if err := dec.ReadMsg(&msg); err != nil {
				l.c.L.Lock()
				if l.err == nil {
					l.err = err
				}
				l.c.L.Unlock()

				l.c.Broadcast()
				return

			}

			l.c.L.Lock()
			l.logs = append(l.logs, &msg)
			l.c.L.Unlock()
			l.c.Broadcast()
		}

	}()
	return nil
}

func (l *mockLoggingPlugin) StopLogging(file string) error {
	l.c.L.Lock()
	if l.err == nil {
		l.err = io.EOF
	}
	l.c.L.Unlock()
	l.c.Broadcast()
	return nil
}

func (l *mockLoggingPlugin) Capabilities() (cap Capability, err error) {
	return Capability{ReadLogs: true}, nil
}

func (l *mockLoggingPlugin) ReadLogs(info Info, config ReadConfig) (io.ReadCloser, error) {
	r, w := io.Pipe()

	go func() {
		var idx int
		enc := logdriver.NewLogEntryEncoder(w)

		l.c.L.Lock()
		defer l.c.L.Unlock()
		for {
			if l.err != nil {
				w.Close()
				return
			}

			if idx >= len(l.logs) {
				if !config.Follow {
					w.Close()
					return
				}

				l.c.Wait()
				continue
			}

			if err := enc.Encode(l.logs[idx]); err != nil {
				w.CloseWithError(err)
				return
			}
			idx++
		}
	}()

	return r, nil
}

func (l *mockLoggingPlugin) waitLen(i int) {
	l.c.L.Lock()
	defer l.c.L.Unlock()
	for len(l.logs) < i {
		l.c.Wait()
	}
}

func (l *mockLoggingPlugin) check(t *testing.T) {
	if l.err != nil && l.err != io.EOF {
		t.Fatal(l.err)
	}
}

func newMockPluginAdapter(plugin *mockLoggingPlugin) Logger {
	enc := logdriver.NewLogEntryEncoder(plugin)
	a := &pluginAdapterWithRead{
		&pluginAdapter{
			plugin: plugin,
			stream: plugin,
			enc:    enc,
		},
	}
	a.plugin.StartLogging("", Info{})
	return a
}

func TestAdapterReadLogs(t *testing.T) {
	plugin := newMockLoggingPlugin()
	l := newMockPluginAdapter(plugin)

	testMsg := []Message{
		{Line: []byte("Are you the keymaker?"), Timestamp: time.Now()},
		{Line: []byte("Follow the white rabbit"), Timestamp: time.Now()},
	}
	for _, msg := range testMsg {
		m := msg.copy()
		assert.Check(t, l.Log(m))
	}

	// Wait until messages are read into plugin
	plugin.waitLen(len(testMsg))

	lr, ok := l.(LogReader)
	assert.Check(t, ok, "Logger does not implement LogReader")

	lw := lr.ReadLogs(ReadConfig{})

	for _, x := range testMsg {
		select {
		case msg := <-lw.Msg:
			testMessageEqual(t, &x, msg)
		case <-time.After(10 * time.Second):
			t.Fatal("timeout reading logs")
		}
	}

	select {
	case _, ok := <-lw.Msg:
		assert.Check(t, !ok, "expected message channel to be closed")
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for message channel to close")

	}
	lw.Close()

	lw = lr.ReadLogs(ReadConfig{Follow: true})
	for _, x := range testMsg {
		select {
		case msg := <-lw.Msg:
			testMessageEqual(t, &x, msg)
		case <-time.After(10 * time.Second):
			t.Fatal("timeout reading logs")
		}
	}

	x := Message{Line: []byte("Too infinity and beyond!"), Timestamp: time.Now()}
	assert.Check(t, l.Log(x.copy()))

	select {
	case msg, ok := <-lw.Msg:
		assert.Check(t, ok, "message channel unexpectedly closed")
		testMessageEqual(t, &x, msg)
	case <-time.After(10 * time.Second):
		t.Fatal("timeout reading logs")
	}

	l.Close()
	select {
	case msg, ok := <-lw.Msg:
		assert.Check(t, !ok, "expected message channel to be closed")
		assert.Check(t, is.Nil(msg))
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for logger to close")
	}

	plugin.check(t)
}

func testMessageEqual(t *testing.T, a, b *Message) {
	assert.Check(t, is.DeepEqual(a.Line, b.Line))
	assert.Check(t, is.DeepEqual(a.Timestamp.UnixNano(), b.Timestamp.UnixNano()))
	assert.Check(t, is.Equal(a.Source, b.Source))
}
