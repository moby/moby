package beam

import (
	"github.com/dotcloud/docker/pkg/beam/data"
	"testing"
)

func TestSendConn(t *testing.T) {
	a, b, err := USocketPair()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	defer b.Close()
	go func() {
		conn, err := SendConn(a, data.Empty().Set("type", "connection").Bytes())
		if err != nil {
			t.Fatal(err)
		}
		if err := conn.Send(data.Empty().Set("foo", "bar").Bytes(), nil); err != nil {
			t.Fatal(err)
		}
		conn.CloseWrite()
	}()
	payload, conn, err := ReceiveConn(b)
	if err != nil {
		t.Fatal(err)
	}
	if val := data.Message(string(payload)).Get("type"); val == nil || val[0] != "connection" {
		t.Fatalf("%v != %v\n", val, "connection")
	}
	msg, _, err := conn.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if val := data.Message(string(msg)).Get("foo"); val == nil || val[0] != "bar" {
		t.Fatalf("%v != %v\n", val, "bar")
	}
}
