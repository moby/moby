package serf

import (
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/testutil"
	"reflect"
	"testing"
)

func TestDelegate_impl(t *testing.T) {
	var raw interface{}
	raw = new(delegate)
	if _, ok := raw.(memberlist.Delegate); !ok {
		t.Fatal("should be an Delegate")
	}
}

func TestDelegate_NodeMeta_Old(t *testing.T) {
	c := testConfig()
	c.ProtocolVersion = 2
	c.Tags["role"] = "test"
	d := &delegate{&Serf{config: c}}
	meta := d.NodeMeta(32)

	if !reflect.DeepEqual(meta, []byte("test")) {
		t.Fatalf("bad meta data: %v", meta)
	}

	out := d.serf.decodeTags(meta)
	if out["role"] != "test" {
		t.Fatalf("bad meta data: %v", meta)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic")
		}
	}()
	d.NodeMeta(1)
}

func TestDelegate_NodeMeta_New(t *testing.T) {
	c := testConfig()
	c.ProtocolVersion = 3
	c.Tags["role"] = "test"
	d := &delegate{&Serf{config: c}}
	meta := d.NodeMeta(32)

	out := d.serf.decodeTags(meta)
	if out["role"] != "test" {
		t.Fatalf("bad meta data: %v", meta)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic")
		}
	}()
	d.NodeMeta(1)
}

// internals
func TestDelegate_LocalState(t *testing.T) {
	c1 := testConfig()
	s1, err := Create(c1)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	c2 := testConfig()
	s2, err := Create(c2)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s2.Shutdown()

	testutil.Yield()

	_, err = s1.Join([]string{c2.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	err = s1.UserEvent("test", []byte("test"), false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	_, err = s1.Query("foo", nil, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// s2 can leave now
	s2.Leave()

	// Do a state dump
	d := c1.MemberlistConfig.Delegate
	buf := d.LocalState(false)

	// Verify
	if messageType(buf[0]) != messagePushPullType {
		t.Fatalf("bad message type")
	}

	// Attempt a decode
	pp := messagePushPull{}
	if err := decodeMessage(buf[1:], &pp); err != nil {
		t.Fatalf("decode failed %v", err)
	}

	// Verify lamport
	if pp.LTime != s1.clock.Time() {
		t.Fatalf("clock mismatch")
	}

	// Verify the status
	if len(pp.StatusLTimes) != 2 {
		t.Fatalf("missing ltimes")
	}

	if len(pp.LeftMembers) != 1 {
		t.Fatalf("missing left members")
	}

	if pp.EventLTime != s1.eventClock.Time() {
		t.Fatalf("clock mismatch")
	}

	if len(pp.Events) != s1.config.EventBuffer {
		t.Fatalf("should send full event buffer")
	}

	if pp.QueryLTime != s1.queryClock.Time() {
		t.Fatalf("clock mismatch")
	}
}

// internals
func TestDelegate_MergeRemoteState(t *testing.T) {
	c1 := testConfig()
	s1, err := Create(c1)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	// Do a state dump
	d := c1.MemberlistConfig.Delegate

	// Make a fake push pull
	pp := messagePushPull{
		LTime: 42,
		StatusLTimes: map[string]LamportTime{
			"test": 20,
			"foo":  15,
		},
		LeftMembers: []string{"foo"},
		EventLTime:  50,
		Events: []*userEvents{
			&userEvents{
				LTime: 45,
				Events: []userEvent{
					userEvent{
						Name:    "test",
						Payload: nil,
					},
				},
			},
		},
		QueryLTime: 100,
	}

	buf, err := encodeMessage(messagePushPullType, &pp)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Merge in fake state
	d.MergeRemoteState(buf, false)

	// Verify lamport
	if s1.clock.Time() != 42 {
		t.Fatalf("clock mismatch")
	}

	// Verify pending join for test
	if s1.recentJoin[0].Node != "test" || s1.recentJoin[0].LTime != 20 {
		t.Fatalf("bad recent join")
	}

	// Verify pending leave for foo
	if s1.recentLeave[0].Node != "foo" || s1.recentLeave[0].LTime != 15 {
		t.Fatalf("bad recent leave")
	}

	// Very event time
	if s1.eventClock.Time() != 50 {
		t.Fatalf("bad event clock")
	}

	if s1.eventBuffer[45] == nil {
		t.Fatalf("missing event buffer for time")
	}
	if s1.eventBuffer[45].Events[0].Name != "test" {
		t.Fatalf("missing event")
	}

	if s1.queryClock.Time() != 100 {
		t.Fatalf("bad query clock")
	}
}
