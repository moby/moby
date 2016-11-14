package logger

import (
	"bytes"
	"encoding/binary"
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/docker/docker/api/types/plugins/logdriver"
	protoio "github.com/gogo/protobuf/io"
)

// mockLoggingPlugin implements the loggingPlugin interface for testing purposes
// it only supports a single log stream
type mockLoggingPlugin struct {
	inStream io.ReadCloser
	f        *os.File
	closed   chan struct{}
	t        *testing.T
}

func (l *mockLoggingPlugin) StartLogging(file string, info Info) error {
	go func() {
		io.Copy(l.f, l.inStream)
		close(l.closed)
	}()
	return nil
}

func (l *mockLoggingPlugin) StopLogging(file string) error {
	l.inStream.Close()
	l.f.Close()
	os.Remove(l.f.Name())
	return nil
}

func (l *mockLoggingPlugin) Capabilities() (cap Capability, err error) {
	return Capability{ReadLogs: true}, nil
}

func (l *mockLoggingPlugin) ReadLogs(info Info, config ReadConfig) (io.ReadCloser, error) {
	r, w := io.Pipe()
	f, err := os.Open(l.f.Name())
	if err != nil {
		return nil, err
	}
	go func() {
		defer f.Close()
		dec := protoio.NewUint32DelimitedReader(f, binary.BigEndian, 1e6)
		enc := logdriver.NewLogEntryEncoder(w)

		for {
			select {
			case <-l.closed:
				w.Close()
				return
			default:
			}

			var msg logdriver.LogEntry
			if err := dec.ReadMsg(&msg); err != nil {
				if err == io.EOF {
					if !config.Follow {
						w.Close()
						return
					}
					dec = protoio.NewUint32DelimitedReader(f, binary.BigEndian, 1e6)
					continue
				}

				l.t.Fatal(err)
				continue
			}

			if err := enc.Encode(&msg); err != nil {
				w.CloseWithError(err)
				return
			}
		}
	}()

	return r, nil
}

func newMockPluginAdapter(t *testing.T) Logger {
	r, w := io.Pipe()
	f, err := ioutil.TempFile("", "mock-plugin-adapter")
	if err != nil {
		t.Fatal(err)
	}
	enc := logdriver.NewLogEntryEncoder(w)
	a := &pluginAdapterWithRead{
		&pluginAdapter{
			plugin: &mockLoggingPlugin{
				inStream: r,
				f:        f,
				closed:   make(chan struct{}),
				t:        t,
			},
			stream: w,
			enc:    enc,
		},
	}
	a.plugin.StartLogging("", Info{})
	return a
}

func TestAdapterReadLogs(t *testing.T) {
	l := newMockPluginAdapter(t)

	testMsg := []Message{
		{Line: []byte("Are you the keymaker?"), Timestamp: time.Now()},
		{Line: []byte("Follow the white rabbit"), Timestamp: time.Now()},
	}
	for _, msg := range testMsg {
		m := msg.copy()
		if err := l.Log(m); err != nil {
			t.Fatal(err)
		}
	}

	lr, ok := l.(LogReader)
	if !ok {
		t.Fatal("expected log reader")
	}

	lw := lr.ReadLogs(ReadConfig{})

	for _, x := range testMsg {
		select {
		case msg := <-lw.Msg:
			testMessageEqual(t, &x, msg)
		case <-time.After(10 * time.Millisecond):
			t.Fatal("timeout reading logs")
		}
	}

	select {
	case _, ok := <-lw.Msg:
		if ok {
			t.Fatal("expected message channel to be closed")
		}
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

	if err := l.Log(x.copy()); err != nil {
		t.Fatal(err)
	}

	select {
	case msg, ok := <-lw.Msg:
		if !ok {
			t.Fatal("message channel unexpectedly closed")
		}
		testMessageEqual(t, &x, msg)
	case <-time.After(10 * time.Second):
		t.Fatal("timeout reading logs")
	}

	l.Close()
	select {
	case msg, ok := <-lw.Msg:
		if ok {
			t.Fatal("expected message channel to be closed")
		}
		if msg != nil {
			t.Fatal("expected nil message")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for logger to close")
	}
}

func testMessageEqual(t *testing.T, a, b *Message) {
	_, _, n, _ := runtime.Caller(1)
	errFmt := "line %d: expected same messages:\nwant: %+v\nhave: %+v"

	if !bytes.Equal(a.Line, b.Line) {
		t.Fatalf(errFmt, n, *a, *b)
	}

	if a.Timestamp.UnixNano() != b.Timestamp.UnixNano() {
		t.Fatalf(errFmt, n, *a, *b)
	}

	if a.Source != b.Source {
		t.Fatalf(errFmt, n, *a, *b)
	}
}
