package memberlist

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestEncodeDecode(t *testing.T) {
	msg := &ping{SeqNo: 100}
	buf, err := encode(pingMsg, msg)
	if err != nil {
		t.Fatalf("unexpected err: %s", err)
	}
	var out ping
	if err := decode(buf.Bytes()[1:], &out); err != nil {
		t.Fatalf("unexpected err: %s", err)
	}
	if msg.SeqNo != out.SeqNo {
		t.Fatalf("bad sequence no")
	}
}

func TestRandomOffset(t *testing.T) {
	vals := make(map[int]struct{})
	for i := 0; i < 100; i++ {
		offset := randomOffset(2 << 30)
		if _, ok := vals[offset]; ok {
			t.Fatalf("got collision")
		}
		vals[offset] = struct{}{}
	}
}

func TestRandomOffset_Zero(t *testing.T) {
	offset := randomOffset(0)
	if offset != 0 {
		t.Fatalf("bad offset")
	}
}

func TestSuspicionTimeout(t *testing.T) {
	timeout := suspicionTimeout(3, 10, time.Second)
	if timeout != 6*time.Second {
		t.Fatalf("bad timeout")
	}
}

func TestRetransmitLimit(t *testing.T) {
	lim := retransmitLimit(3, 0)
	if lim != 0 {
		t.Fatalf("bad val %v", lim)
	}
	lim = retransmitLimit(3, 1)
	if lim != 3 {
		t.Fatalf("bad val %v", lim)
	}
	lim = retransmitLimit(3, 99)
	if lim != 6 {
		t.Fatalf("bad val %v", lim)
	}
}

func TestShuffleNodes(t *testing.T) {
	orig := []*nodeState{
		&nodeState{
			State: stateDead,
		},
		&nodeState{
			State: stateAlive,
		},
		&nodeState{
			State: stateAlive,
		},
		&nodeState{
			State: stateDead,
		},
		&nodeState{
			State: stateAlive,
		},
		&nodeState{
			State: stateAlive,
		},
		&nodeState{
			State: stateDead,
		},
		&nodeState{
			State: stateAlive,
		},
	}
	nodes := make([]*nodeState, len(orig))
	copy(nodes[:], orig[:])

	if !reflect.DeepEqual(nodes, orig) {
		t.Fatalf("should match")
	}

	shuffleNodes(nodes)

	if reflect.DeepEqual(nodes, orig) {
		t.Fatalf("should not match")
	}
}

func TestPushPullScale(t *testing.T) {
	sec := time.Second
	for i := 0; i <= 32; i++ {
		if s := pushPullScale(sec, i); s != sec {
			t.Fatalf("Bad time scale: %v", s)
		}
	}
	for i := 33; i <= 64; i++ {
		if s := pushPullScale(sec, i); s != 2*sec {
			t.Fatalf("Bad time scale: %v", s)
		}
	}
	for i := 65; i <= 128; i++ {
		if s := pushPullScale(sec, i); s != 3*sec {
			t.Fatalf("Bad time scale: %v", s)
		}
	}
}

func TestMoveDeadNodes(t *testing.T) {
	nodes := []*nodeState{
		&nodeState{
			State: stateDead,
		},
		&nodeState{
			State: stateAlive,
		},
		&nodeState{
			State: stateAlive,
		},
		&nodeState{
			State: stateDead,
		},
		&nodeState{
			State: stateAlive,
		},
	}

	idx := moveDeadNodes(nodes)
	if idx != 3 {
		t.Fatalf("bad index")
	}
	for i := 0; i < idx; i++ {
		if nodes[i].State != stateAlive {
			t.Fatalf("Bad state %d", i)
		}
	}
	for i := idx; i < len(nodes); i++ {
		if nodes[i].State != stateDead {
			t.Fatalf("Bad state %d", i)
		}
	}
}

func TestKRandomNodes(t *testing.T) {
	nodes := []*nodeState{}
	for i := 0; i < 90; i++ {
		// Half the nodes are in a bad state
		state := stateAlive
		switch i % 3 {
		case 0:
			state = stateAlive
		case 1:
			state = stateSuspect
		case 2:
			state = stateDead
		}
		nodes = append(nodes, &nodeState{
			Node: Node{
				Name: fmt.Sprintf("test%d", i),
			},
			State: state,
		})
	}

	s1 := kRandomNodes(3, []string{"test0"}, nodes)
	s2 := kRandomNodes(3, []string{"test0"}, nodes)
	s3 := kRandomNodes(3, []string{"test0"}, nodes)

	if reflect.DeepEqual(s1, s2) {
		t.Fatalf("unexpected equal")
	}
	if reflect.DeepEqual(s1, s3) {
		t.Fatalf("unexpected equal")
	}
	if reflect.DeepEqual(s2, s3) {
		t.Fatalf("unexpected equal")
	}

	for _, s := range [][]*nodeState{s1, s2, s3} {
		if len(s) != 3 {
			t.Fatalf("bad len")
		}
		for _, n := range s {
			if n.Name == "test0" {
				t.Fatalf("Bad name")
			}
			if n.State != stateAlive {
				t.Fatalf("Bad state")
			}
		}
	}
}

func TestMakeCompoundMessage(t *testing.T) {
	msg := &ping{SeqNo: 100}
	buf, err := encode(pingMsg, msg)
	if err != nil {
		t.Fatalf("unexpected err: %s", err)
	}

	msgs := [][]byte{buf.Bytes(), buf.Bytes(), buf.Bytes()}
	compound := makeCompoundMessage(msgs)

	if compound.Len() != 3*buf.Len()+3*compoundOverhead+compoundHeaderOverhead {
		t.Fatalf("bad len")
	}
}

func TestDecodeCompoundMessage(t *testing.T) {
	msg := &ping{SeqNo: 100}
	buf, err := encode(pingMsg, msg)
	if err != nil {
		t.Fatalf("unexpected err: %s", err)
	}

	msgs := [][]byte{buf.Bytes(), buf.Bytes(), buf.Bytes()}
	compound := makeCompoundMessage(msgs)

	trunc, parts, err := decodeCompoundMessage(compound.Bytes()[1:])
	if err != nil {
		t.Fatalf("unexpected err: %s", err)
	}
	if trunc != 0 {
		t.Fatalf("should not truncate")
	}
	if len(parts) != 3 {
		t.Fatalf("bad parts")
	}
	for _, p := range parts {
		if len(p) != buf.Len() {
			t.Fatalf("bad part len")
		}
	}
}

func TestDecodeCompoundMessage_Trunc(t *testing.T) {
	msg := &ping{SeqNo: 100}
	buf, err := encode(pingMsg, msg)
	if err != nil {
		t.Fatalf("unexpected err: %s", err)
	}

	msgs := [][]byte{buf.Bytes(), buf.Bytes(), buf.Bytes()}
	compound := makeCompoundMessage(msgs)

	trunc, parts, err := decodeCompoundMessage(compound.Bytes()[1:38])
	if err != nil {
		t.Fatalf("unexpected err: %s", err)
	}
	if trunc != 1 {
		t.Fatalf("truncate: %d", trunc)
	}
	if len(parts) != 2 {
		t.Fatalf("bad parts")
	}
	for _, p := range parts {
		if len(p) != buf.Len() {
			t.Fatalf("bad part len")
		}
	}
}

func TestCompressDecompressPayload(t *testing.T) {
	buf, err := compressPayload([]byte("testing"))
	if err != nil {
		t.Fatalf("unexpected err: %s", err)
	}

	decomp, err := decompressPayload(buf.Bytes()[1:])
	if err != nil {
		t.Fatalf("unexpected err: %s", err)
	}

	if !reflect.DeepEqual(decomp, []byte("testing")) {
		t.Fatalf("bad payload: %v", decomp)
	}
}
