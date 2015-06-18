package serf

import (
	"reflect"
	"testing"
)

func TestQueryFlags(t *testing.T) {
	if queryFlagAck != 1 {
		t.Fatalf("Bad: %v", queryFlagAck)
	}
	if queryFlagNoBroadcast != 2 {
		t.Fatalf("Bad: %v", queryFlagNoBroadcast)
	}
}

func TestEncodeMessage(t *testing.T) {
	in := &messageLeave{Node: "foo"}
	raw, err := encodeMessage(messageLeaveType, in)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if raw[0] != byte(messageLeaveType) {
		t.Fatal("should have type header")
	}

	var out messageLeave
	if err := decodeMessage(raw[1:], &out); err != nil {
		t.Fatalf("err: %s", err)
	}

	if !reflect.DeepEqual(in, &out) {
		t.Fatalf("mis-match")
	}
}

func TestEncodeFilter(t *testing.T) {
	nodes := []string{"foo", "bar"}

	raw, err := encodeFilter(filterNodeType, nodes)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if raw[0] != byte(filterNodeType) {
		t.Fatal("should have type header")
	}

	var out []string
	if err := decodeMessage(raw[1:], &out); err != nil {
		t.Fatalf("err: %s", err)
	}

	if !reflect.DeepEqual(nodes, out) {
		t.Fatalf("mis-match")
	}
}
