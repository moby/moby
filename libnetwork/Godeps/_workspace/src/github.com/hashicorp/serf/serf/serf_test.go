package serf

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/testutil"
)

func testConfig() *Config {
	config := DefaultConfig()
	config.Init()
	config.MemberlistConfig.BindAddr = testutil.GetBindAddr().String()

	// Set probe intervals that are aggressive for finding bad nodes
	config.MemberlistConfig.GossipInterval = 5 * time.Millisecond
	config.MemberlistConfig.ProbeInterval = 50 * time.Millisecond
	config.MemberlistConfig.ProbeTimeout = 25 * time.Millisecond
	config.MemberlistConfig.SuspicionMult = 1

	config.NodeName = fmt.Sprintf("Node %s", config.MemberlistConfig.BindAddr)

	// Set a short reap interval so that it can run during the test
	config.ReapInterval = 1 * time.Second

	// Set a short reconnect interval so that it can run a lot during tests
	config.ReconnectInterval = 100 * time.Millisecond

	// Set basically zero on the reconnect/tombstone timeouts so that
	// they're removed on the first ReapInterval.
	config.ReconnectTimeout = 1 * time.Microsecond
	config.TombstoneTimeout = 1 * time.Microsecond

	return config
}

// testMember tests that a member in a list is in a given state.
func testMember(t *testing.T, members []Member, name string, status MemberStatus) {
	for _, m := range members {
		if m.Name == name {
			if m.Status != status {
				panic(fmt.Sprintf("bad state for %s: %d", name, m.Status))
			}
			return
		}
	}

	if status == StatusNone {
		// We didn't expect to find it
		return
	}

	panic(fmt.Sprintf("node not found: %s", name))
}

func TestCreate_badProtocolVersion(t *testing.T) {
	cases := []struct {
		version uint8
		err     bool
	}{
		{ProtocolVersionMin, false},
		{ProtocolVersionMax, false},
		// TODO(mitchellh): uncommon when we're over 0
		//{ProtocolVersionMin - 1, true},
		{ProtocolVersionMax + 1, true},
		{ProtocolVersionMax - 1, false},
	}

	for _, tc := range cases {
		c := testConfig()
		c.ProtocolVersion = tc.version
		s, err := Create(c)
		if tc.err && err == nil {
			t.Errorf("Should've failed with version: %d", tc.version)
		} else if !tc.err && err != nil {
			t.Errorf("Version '%d' error: %s", tc.version, err)
		}

		if err == nil {
			s.Shutdown()
		}
	}
}

func TestSerf_eventsFailed(t *testing.T) {
	// Create the s1 config with an event channel so we can listen
	eventCh := make(chan Event, 4)
	s1Config := testConfig()
	s1Config.EventCh = eventCh

	s2Config := testConfig()

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()

	testutil.Yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}

	time.Sleep(1 * time.Second)

	// Since s2 shutdown, we check the events to make sure we got failures.
	testEvents(t, eventCh, s2Config.NodeName,
		[]EventType{EventMemberJoin, EventMemberFailed, EventMemberReap})
}

func TestSerf_eventsJoin(t *testing.T) {
	// Create the s1 config with an event channel so we can listen
	eventCh := make(chan Event, 4)
	s1Config := testConfig()
	s1Config.EventCh = eventCh

	s2Config := testConfig()

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()

	testutil.Yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	testEvents(t, eventCh, s2Config.NodeName,
		[]EventType{EventMemberJoin})
}

func TestSerf_eventsLeave(t *testing.T) {
	// Create the s1 config with an event channel so we can listen
	eventCh := make(chan Event, 4)
	s1Config := testConfig()
	s1Config.EventCh = eventCh

	s2Config := testConfig()

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()

	testutil.Yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if err := s2.Leave(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Now that s2 has left, we check the events to make sure we got
	// a leave event in s1 about the leave.
	testEvents(t, eventCh, s2Config.NodeName,
		[]EventType{EventMemberJoin, EventMemberLeave})
}

func TestSerf_eventsUser(t *testing.T) {
	// Create the s1 config with an event channel so we can listen
	eventCh := make(chan Event, 4)
	s1Config := testConfig()
	s2Config := testConfig()
	s2Config.EventCh = eventCh

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()

	testutil.Yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Fire a user event
	if err := s1.UserEvent("event!", []byte("test"), false); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Fire a user event
	if err := s1.UserEvent("second", []byte("foobar"), false); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// check the events to make sure we got
	// a leave event in s1 about the leave.
	testUserEvents(t, eventCh,
		[]string{"event!", "second"},
		[][]byte{[]byte("test"), []byte("foobar")})
}

func TestSerf_eventsUser_sizeLimit(t *testing.T) {
	// Create the s1 config with an event channel so we can listen
	s1Config := testConfig()
	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	name := "this is too large an event"
	payload := make([]byte, UserEventSizeLimit)
	err = s1.UserEvent(name, payload, false)
	if err == nil {
		t.Fatalf("expect error")
	}
	if !strings.HasPrefix(err.Error(), "user event exceeds limit of ") {
		t.Fatalf("should get size limit error")
	}
}

func TestSerf_joinLeave(t *testing.T) {
	s1Config := testConfig()
	s2Config := testConfig()

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()

	testutil.Yield()

	if len(s1.Members()) != 1 {
		t.Fatalf("s1 members: %d", len(s1.Members()))
	}

	if len(s2.Members()) != 1 {
		t.Fatalf("s2 members: %d", len(s2.Members()))
	}

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if len(s1.Members()) != 2 {
		t.Fatalf("s1 members: %d", len(s1.Members()))
	}

	if len(s2.Members()) != 2 {
		t.Fatalf("s2 members: %d", len(s2.Members()))
	}

	err = s1.Leave()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Give the reaper time to reap nodes
	time.Sleep(s1Config.ReapInterval * 2)

	if len(s1.Members()) != 1 {
		t.Fatalf("s1 members: %d", len(s1.Members()))
	}

	if len(s2.Members()) != 1 {
		t.Fatalf("s2 members: %d", len(s2.Members()))
	}
}

// Bug: GH-58
func TestSerf_leaveRejoinDifferentRole(t *testing.T) {
	s1Config := testConfig()
	s2Config := testConfig()
	s2Config.Tags["role"] = "foo"

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()

	testutil.Yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	err = s2.Leave()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Make s3 look just like s2, but create a new node with a new role
	s3Config := testConfig()
	s3Config.MemberlistConfig.BindAddr = s2Config.MemberlistConfig.BindAddr
	s3Config.NodeName = s2Config.NodeName
	s3Config.Tags["role"] = "bar"

	s3, err := Create(s3Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s3.Shutdown()

	_, err = s3.Join([]string{s1Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	members := s1.Members()
	if len(members) != 2 {
		t.Fatalf("s1 members: %d", len(s1.Members()))
	}

	var member *Member = nil
	for _, m := range members {
		if m.Name == s3Config.NodeName {
			member = &m
			break
		}
	}

	if member == nil {
		t.Fatalf("couldn't find member")
	}

	if member.Tags["role"] != s3Config.Tags["role"] {
		t.Fatalf("bad role: %s", member.Tags["role"])
	}
}

func TestSerf_reconnect(t *testing.T) {
	eventCh := make(chan Event, 64)
	s1Config := testConfig()
	s1Config.EventCh = eventCh

	s2Config := testConfig()
	s2Addr := s2Config.MemberlistConfig.BindAddr
	s2Name := s2Config.NodeName

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()

	testutil.Yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Now force the shutdown of s2 so it appears to fail.
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}

	time.Sleep(s2Config.MemberlistConfig.ProbeInterval * 5)

	// Bring back s2 by mimicking its name and address
	s2Config = testConfig()
	s2Config.MemberlistConfig.BindAddr = s2Addr
	s2Config.NodeName = s2Name
	s2, err = Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	time.Sleep(s1Config.ReconnectInterval * 5)

	testEvents(t, eventCh, s2Name,
		[]EventType{EventMemberJoin, EventMemberFailed, EventMemberJoin})
}

func TestSerf_reconnect_sameIP(t *testing.T) {
	eventCh := make(chan Event, 64)
	s1Config := testConfig()
	s1Config.EventCh = eventCh

	s2Config := testConfig()
	s2Config.MemberlistConfig.BindAddr = s1Config.MemberlistConfig.BindAddr
	s2Config.MemberlistConfig.BindPort = s1Config.MemberlistConfig.BindPort + 1

	s2Addr := fmt.Sprintf("%s:%d",
		s2Config.MemberlistConfig.BindAddr,
		s2Config.MemberlistConfig.BindPort)
	s2Name := s2Config.NodeName

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()

	testutil.Yield()

	_, err = s1.Join([]string{s2Addr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Now force the shutdown of s2 so it appears to fail.
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}

	time.Sleep(s2Config.MemberlistConfig.ProbeInterval * 5)

	// Bring back s2 by mimicking its name and address
	s2Config = testConfig()
	s2Config.MemberlistConfig.BindAddr = s1Config.MemberlistConfig.BindAddr
	s2Config.MemberlistConfig.BindPort = s1Config.MemberlistConfig.BindPort + 1
	s2Config.NodeName = s2Name
	s2, err = Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	time.Sleep(s1Config.ReconnectInterval * 5)

	testEvents(t, eventCh, s2Name,
		[]EventType{EventMemberJoin, EventMemberFailed, EventMemberJoin})
}

func TestSerf_role(t *testing.T) {
	s1Config := testConfig()
	s2Config := testConfig()

	s1Config.Tags["role"] = "web"
	s2Config.Tags["role"] = "lb"

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	members := s1.Members()
	if len(members) != 2 {
		t.Fatalf("should have 2 members")
	}

	roles := make(map[string]string)
	for _, m := range members {
		roles[m.Name] = m.Tags["role"]
	}

	if roles[s1Config.NodeName] != "web" {
		t.Fatalf("bad role for web: %s", roles[s1Config.NodeName])
	}

	if roles[s2Config.NodeName] != "lb" {
		t.Fatalf("bad role for lb: %s", roles[s2Config.NodeName])
	}
}

func TestSerfProtocolVersion(t *testing.T) {
	config := testConfig()
	config.ProtocolVersion = ProtocolVersionMax

	s1, err := Create(config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	actual := s1.ProtocolVersion()
	if actual != ProtocolVersionMax {
		t.Fatalf("bad: %#v", actual)
	}
}

func TestSerfRemoveFailedNode(t *testing.T) {
	s1Config := testConfig()
	s2Config := testConfig()
	s3Config := testConfig()

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s3, err := Create(s3Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()
	defer s3.Shutdown()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	_, err = s1.Join([]string{s3Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Now force the shutdown of s2 so it appears to fail.
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}

	time.Sleep(s2Config.MemberlistConfig.ProbeInterval * 5)

	// Verify that s2 is "failed"
	testMember(t, s1.Members(), s2Config.NodeName, StatusFailed)

	// Now remove the failed node
	if err := s1.RemoveFailedNode(s2Config.NodeName); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Verify that s2 is gone
	testMember(t, s1.Members(), s2Config.NodeName, StatusLeft)
	testMember(t, s3.Members(), s2Config.NodeName, StatusLeft)
}

func TestSerfRemoveFailedNode_ourself(t *testing.T) {
	s1Config := testConfig()
	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	testutil.Yield()

	if err := s1.RemoveFailedNode("somebody"); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestSerfState(t *testing.T) {
	s1, err := Create(testConfig())
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	if s1.State() != SerfAlive {
		t.Fatalf("bad state: %d", s1.State())
	}

	if err := s1.Leave(); err != nil {
		t.Fatalf("err: %s", err)
	}

	if s1.State() != SerfLeft {
		t.Fatalf("bad state: %d", s1.State())
	}

	if err := s1.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}

	if s1.State() != SerfShutdown {
		t.Fatalf("bad state: %d", s1.State())
	}
}

func TestSerf_ReapHandler_Shutdown(t *testing.T) {
	s, err := Create(testConfig())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	go func() {
		s.Shutdown()
		time.Sleep(time.Millisecond)
		t.Fatalf("timeout")
	}()
	s.handleReap()
}

func TestSerf_ReapHandler(t *testing.T) {
	c := testConfig()
	c.ReapInterval = time.Nanosecond
	c.TombstoneTimeout = time.Second * 6
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	m := Member{}
	s.leftMembers = []*memberState{
		&memberState{m, 0, time.Now()},
		&memberState{m, 0, time.Now().Add(-5 * time.Second)},
		&memberState{m, 0, time.Now().Add(-10 * time.Second)},
	}

	go func() {
		time.Sleep(time.Millisecond)
		s.Shutdown()
	}()

	s.handleReap()

	if len(s.leftMembers) != 2 {
		t.Fatalf("should be shorter")
	}
}

func TestSerf_Reap(t *testing.T) {
	s, err := Create(testConfig())
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	m := Member{}
	old := []*memberState{
		&memberState{m, 0, time.Now()},
		&memberState{m, 0, time.Now().Add(-5 * time.Second)},
		&memberState{m, 0, time.Now().Add(-10 * time.Second)},
	}

	old = s.reap(old, time.Second*6)
	if len(old) != 2 {
		t.Fatalf("should be shorter")
	}
}

func TestRemoveOldMember(t *testing.T) {
	old := []*memberState{
		&memberState{Member: Member{Name: "foo"}},
		&memberState{Member: Member{Name: "bar"}},
		&memberState{Member: Member{Name: "baz"}},
	}

	old = removeOldMember(old, "bar")
	if len(old) != 2 {
		t.Fatalf("should be shorter")
	}
	if old[1].Name == "bar" {
		t.Fatalf("should remove old member")
	}
}

func TestRecentIntent(t *testing.T) {
	if recentIntent(nil, "foo") != nil {
		t.Fatalf("should get nil on empty recent")
	}
	if recentIntent([]nodeIntent{}, "foo") != nil {
		t.Fatalf("should get nil on empty recent")
	}

	recent := []nodeIntent{
		nodeIntent{1, "foo"},
		nodeIntent{2, "bar"},
		nodeIntent{3, "baz"},
		nodeIntent{4, "bar"},
		nodeIntent{0, "bar"},
		nodeIntent{5, "bar"},
	}

	if r := recentIntent(recent, "bar"); r.LTime != 4 {
		t.Fatalf("bad time for bar")
	}

	if r := recentIntent(recent, "tubez"); r != nil {
		t.Fatalf("got result for tubez")
	}
}

func TestMemberStatus_String(t *testing.T) {
	status := []MemberStatus{StatusNone, StatusAlive, StatusLeaving, StatusLeft, StatusFailed}
	expect := []string{"none", "alive", "leaving", "left", "failed"}

	for idx, s := range status {
		if s.String() != expect[idx] {
			t.Fatalf("got string %v, expected %v", s.String(), expect[idx])
		}
	}

	other := MemberStatus(100)
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic")
		}
	}()
	other.String()
}

func TestSerf_joinLeaveJoin(t *testing.T) {
	s1Config := testConfig()
	s1Config.ReapInterval = 10 * time.Second
	s2Config := testConfig()
	s2Config.ReapInterval = 10 * time.Second

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if len(s1.Members()) != 1 {
		t.Fatalf("s1 members: %d", len(s1.Members()))
	}

	if len(s2.Members()) != 1 {
		t.Fatalf("s2 members: %d", len(s2.Members()))
	}

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if len(s1.Members()) != 2 {
		t.Fatalf("s1 members: %d", len(s1.Members()))
	}

	if len(s2.Members()) != 2 {
		t.Fatalf("s2 members: %d", len(s2.Members()))
	}

	// Leave and shutdown
	err = s2.Leave()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	s2.Shutdown()

	// Give the reaper time to reap nodes
	time.Sleep(s1Config.MemberlistConfig.ProbeInterval * 5)

	// s1 should see the node as having left
	mems := s1.Members()
	anyLeft := false
	for _, m := range mems {
		if m.Status == StatusLeft {
			anyLeft = true
			break
		}
	}
	if !anyLeft {
		t.Fatalf("node should have left!")
	}

	// Bring node 2 back
	s2, err = Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s2.Shutdown()

	testutil.Yield()

	// Re-attempt the join
	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Should be back to both members
	if len(s1.Members()) != 2 {
		t.Fatalf("s1 members: %d", len(s1.Members()))
	}

	if len(s2.Members()) != 2 {
		t.Fatalf("s2 members: %d", len(s2.Members()))
	}

	// s1 should see the node as alive
	mems = s1.Members()
	anyLeft = false
	for _, m := range mems {
		if m.Status == StatusLeft {
			anyLeft = true
			break
		}
	}
	if anyLeft {
		t.Fatalf("all nodes should be alive!")
	}
}

func TestSerf_Join_IgnoreOld(t *testing.T) {
	// Create the s1 config with an event channel so we can listen
	eventCh := make(chan Event, 4)
	s1Config := testConfig()
	s2Config := testConfig()
	s2Config.EventCh = eventCh

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()

	testutil.Yield()

	// Fire a user event
	if err := s1.UserEvent("event!", []byte("test"), false); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Fire a user event
	if err := s1.UserEvent("second", []byte("foobar"), false); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// join with ignoreOld set to true! should not get events
	_, err = s2.Join([]string{s1Config.MemberlistConfig.BindAddr}, true)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// check the events to make sure we got nothing
	testUserEvents(t, eventCh, []string{}, [][]byte{})
}

func TestSerf_SnapshotRecovery(t *testing.T) {
	td, err := ioutil.TempDir("", "serf")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(td)

	s1Config := testConfig()
	s2Config := testConfig()
	s2Config.SnapshotPath = td + "snap"

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s2.Shutdown()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Fire a user event
	if err := s1.UserEvent("event!", []byte("test"), false); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Now force the shutdown of s2 so it appears to fail.
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}
	time.Sleep(s2Config.MemberlistConfig.ProbeInterval * 10)

	// Verify that s2 is "failed"
	testMember(t, s1.Members(), s2Config.NodeName, StatusFailed)

	// Now remove the failed node
	if err := s1.RemoveFailedNode(s2Config.NodeName); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Verify that s2 is gone
	testMember(t, s1.Members(), s2Config.NodeName, StatusLeft)

	// Listen for events
	eventCh := make(chan Event, 4)
	s2Config.EventCh = eventCh

	// Restart s2 from the snapshot now!
	s2, err = Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s2.Shutdown()

	// Wait for the node to auto rejoin
	start := time.Now()
	for time.Now().Sub(start) < time.Second {
		members := s1.Members()
		if len(members) == 2 && members[0].Status == StatusAlive && members[1].Status == StatusAlive {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify that s2 is "alive"
	testMember(t, s1.Members(), s2Config.NodeName, StatusAlive)
	testMember(t, s2.Members(), s1Config.NodeName, StatusAlive)

	// Check the events to make sure we got nothing
	testUserEvents(t, eventCh, []string{}, [][]byte{})
}

func TestSerf_Leave_SnapshotRecovery(t *testing.T) {
	td, err := ioutil.TempDir("", "serf")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(td)

	s1Config := testConfig()
	s2Config := testConfig()
	s2Config.SnapshotPath = td + "snap"

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s2.Shutdown()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if err := s2.Leave(); err != nil {
		t.Fatalf("err: %s", err)
	}
	if err := s2.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}
	time.Sleep(s2Config.MemberlistConfig.ProbeInterval * 5)

	// Verify that s2 is "left"
	testMember(t, s1.Members(), s2Config.NodeName, StatusLeft)

	// Restart s2 from the snapshot now!
	s2Config.EventCh = nil
	s2, err = Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s2.Shutdown()

	// Wait for the node to auto rejoin
	testutil.Yield()

	// Verify that s2 is didn't join
	testMember(t, s1.Members(), s2Config.NodeName, StatusLeft)
	if len(s2.Members()) != 1 {
		t.Fatalf("bad members: %#v", s2.Members())
	}
}

func TestSerf_SetTags(t *testing.T) {
	eventCh := make(chan Event, 4)
	s1Config := testConfig()
	s1Config.EventCh = eventCh
	s2Config := testConfig()

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s2.Shutdown()
	testutil.Yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	testutil.Yield()

	// Update the tags
	if err := s1.SetTags(map[string]string{"port": "8000"}); err != nil {
		t.Fatalf("err: %s", err)
	}

	if err := s2.SetTags(map[string]string{"datacenter": "east-aws"}); err != nil {
		t.Fatalf("err: %s", err)
	}
	testutil.Yield()

	// Since s2 shutdown, we check the events to make sure we got failures.
	testEvents(t, eventCh, s2Config.NodeName,
		[]EventType{EventMemberJoin, EventMemberUpdate})

	// Verify the new tags
	m1m := s1.Members()
	m1m_tags := make(map[string]map[string]string)
	for _, m := range m1m {
		m1m_tags[m.Name] = m.Tags
	}

	if m := m1m_tags[s1.config.NodeName]; m["port"] != "8000" {
		t.Fatalf("bad: %v", m1m_tags)
	}

	if m := m1m_tags[s2.config.NodeName]; m["datacenter"] != "east-aws" {
		t.Fatalf("bad: %v", m1m_tags)
	}

	m2m := s2.Members()
	m2m_tags := make(map[string]map[string]string)
	for _, m := range m2m {
		m2m_tags[m.Name] = m.Tags
	}

	if m := m2m_tags[s1.config.NodeName]; m["port"] != "8000" {
		t.Fatalf("bad: %v", m1m_tags)
	}

	if m := m2m_tags[s2.config.NodeName]; m["datacenter"] != "east-aws" {
		t.Fatalf("bad: %v", m1m_tags)
	}
}

func TestSerf_Query(t *testing.T) {
	eventCh := make(chan Event, 4)
	s1Config := testConfig()
	s1Config.EventCh = eventCh
	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	// Listen for the query
	go func() {
		for {
			select {
			case e := <-eventCh:
				if e.EventType() != EventQuery {
					continue
				}
				q := e.(*Query)
				if err := q.Respond([]byte("test")); err != nil {
					t.Fatalf("err: %s", err)
				}
				return
			case <-time.After(time.Second):
				t.Fatalf("timeout")
			}
		}
	}()

	s2Config := testConfig()
	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s2.Shutdown()
	testutil.Yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	testutil.Yield()

	// Start a query from s2
	params := s2.DefaultQueryParams()
	params.RequestAck = true
	resp, err := s2.Query("load", []byte("sup girl"), params)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	var acks []string
	var responses []string

	ackCh := resp.AckCh()
	respCh := resp.ResponseCh()
	for i := 0; i < 3; i++ {
		select {
		case a := <-ackCh:
			acks = append(acks, a)

		case r := <-respCh:
			if r.From != s1Config.NodeName {
				t.Fatalf("bad: %v", r)
			}
			if string(r.Payload) != "test" {
				t.Fatalf("bad: %v", r)
			}
			responses = append(responses, r.From)

		case <-time.After(time.Second):
			t.Fatalf("timeout")
		}
	}

	if len(acks) != 2 {
		t.Fatalf("missing acks: %v", acks)
	}
	if len(responses) != 1 {
		t.Fatalf("missing responses: %v", responses)
	}
}

func TestSerf_Query_Filter(t *testing.T) {
	eventCh := make(chan Event, 4)
	s1Config := testConfig()
	s1Config.EventCh = eventCh
	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	// Listen for the query
	go func() {
		for {
			select {
			case e := <-eventCh:
				if e.EventType() != EventQuery {
					continue
				}
				q := e.(*Query)
				if err := q.Respond([]byte("test")); err != nil {
					t.Fatalf("err: %s", err)
				}
				return
			case <-time.After(time.Second):
				t.Fatalf("timeout")
			}
		}
	}()

	s2Config := testConfig()
	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s2.Shutdown()
	testutil.Yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	testutil.Yield()

	// Filter to only s1!
	params := s2.DefaultQueryParams()
	params.FilterNodes = []string{s1Config.NodeName}
	params.RequestAck = true

	// Start a query from s2
	resp, err := s2.Query("load", []byte("sup girl"), params)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	var acks []string
	var responses []string

	ackCh := resp.AckCh()
	respCh := resp.ResponseCh()
	for i := 0; i < 2; i++ {
		select {
		case a := <-ackCh:
			acks = append(acks, a)

		case r := <-respCh:
			if r.From != s1Config.NodeName {
				t.Fatalf("bad: %v", r)
			}
			if string(r.Payload) != "test" {
				t.Fatalf("bad: %v", r)
			}
			responses = append(responses, r.From)

		case <-time.After(time.Second):
			t.Fatalf("timeout")
		}
	}

	if len(acks) != 1 {
		t.Fatalf("missing acks: %v", acks)
	}
	if len(responses) != 1 {
		t.Fatalf("missing responses: %v", responses)
	}
}

func TestSerf_Query_sizeLimit(t *testing.T) {
	// Create the s1 config with an event channel so we can listen
	s1Config := testConfig()
	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	name := "this is too large a query"
	payload := make([]byte, QuerySizeLimit)
	_, err = s1.Query(name, payload, nil)
	if err == nil {
		t.Fatalf("should get error")
	}
	if !strings.HasPrefix(err.Error(), "query exceeds limit of ") {
		t.Fatalf("should get size limit error: %v", err)
	}
}

func TestSerf_NameResolution(t *testing.T) {
	// Create the s1 config with an event channel so we can listen
	s1Config := testConfig()
	s2Config := testConfig()
	s3Config := testConfig()

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s2.Shutdown()

	// Create an artificial node name conflict!
	s3Config.NodeName = s1Config.NodeName
	s3, err := Create(s3Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s3.Shutdown()

	testutil.Yield()

	// Join s1 to s2 first. s2 should vote for s1 in conflict
	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	_, err = s1.Join([]string{s3Config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Wait for the query period to end
	time.Sleep(s1.DefaultQueryTimeout() * 2)

	// s3 should have shutdown, while s1 is running
	if s1.State() != SerfAlive {
		t.Fatalf("bad: %v", s1.State())
	}
	if s2.State() != SerfAlive {
		t.Fatalf("bad: %v", s2.State())
	}
	if s3.State() != SerfShutdown {
		t.Fatalf("bad: %v", s3.State())
	}
}

func TestSerf_LocalMember(t *testing.T) {
	s1Config := testConfig()
	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	m := s1.LocalMember()
	if m.Name != s1Config.NodeName {
		t.Fatalf("bad: %v", m)
	}
	if !reflect.DeepEqual(m.Tags, s1Config.Tags) {
		t.Fatalf("bad: %v", m)
	}
	if m.Status != StatusAlive {
		t.Fatalf("bad: %v", m)
	}

	newTags := map[string]string{
		"foo":  "bar",
		"test": "ing",
	}
	if err := s1.SetTags(newTags); err != nil {
		t.Fatalf("err: %s", err)
	}

	m = s1.LocalMember()
	if !reflect.DeepEqual(m.Tags, newTags) {
		t.Fatalf("bad: %v", m)
	}
}

func TestSerf_WriteKeyringFile(t *testing.T) {
	existing := "jbuQMI4gMUeh1PPmKOtiBg=="
	newKey := "eodFZZjm7pPwIZ0Miy7boQ=="

	td, err := ioutil.TempDir("", "serf")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(td)

	keyringFile := filepath.Join(td, "tags.json")

	existingBytes, err := base64.StdEncoding.DecodeString(existing)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	keys := [][]byte{existingBytes}
	keyring, err := memberlist.NewKeyring(keys, existingBytes)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s1Config := testConfig()
	s1Config.MemberlistConfig.Keyring = keyring
	s1Config.KeyringFile = keyringFile
	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	manager := s1.KeyManager()

	if _, err := manager.InstallKey(newKey); err != nil {
		t.Fatalf("err: %s", err)
	}

	content, err := ioutil.ReadFile(keyringFile)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) != 4 {
		t.Fatalf("bad: %v", lines)
	}

	// Ensure both the original key and the new key are present in the file
	if !strings.Contains(string(content), existing) {
		t.Fatalf("key not found in keyring file: %s", existing)
	}
	if !strings.Contains(string(content), newKey) {
		t.Fatalf("key not found in keyring file: %s", newKey)
	}

	// Ensure the existing key remains primary. This is in position 1 because
	// the file writer will use json.MarshalIndent(), leaving the first line as
	// the opening bracket.
	if !strings.Contains(lines[1], existing) {
		t.Fatalf("expected key to be primary: %s", existing)
	}

	// Swap primary keys
	if _, err := manager.UseKey(newKey); err != nil {
		t.Fatalf("err: %s", err)
	}

	content, err = ioutil.ReadFile(keyringFile)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	lines = strings.Split(string(content), "\n")
	if len(lines) != 4 {
		t.Fatalf("bad: %v", lines)
	}

	// Key order should have changed in keyring file
	if !strings.Contains(lines[1], newKey) {
		t.Fatalf("expected key to be primary: %s", newKey)
	}

	// Remove the old key
	if _, err := manager.RemoveKey(existing); err != nil {
		t.Fatalf("err: %s", err)
	}

	content, err = ioutil.ReadFile(keyringFile)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	lines = strings.Split(string(content), "\n")
	if len(lines) != 3 {
		t.Fatalf("bad: %v", lines)
	}

	// Only the new key should now be present in the keyring file
	if len(lines) != 3 {
		t.Fatalf("bad: %v", lines)
	}
	if !strings.Contains(lines[1], newKey) {
		t.Fatalf("expected key to be primary: %s", newKey)
	}
}

func TestSerfStats(t *testing.T) {
	config := testConfig()
	s1, err := Create(config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s1.Shutdown()

	stats := s1.Stats()

	expected := map[string]string{
		"event_queue":  "0",
		"event_time":   "1",
		"failed":       "0",
		"intent_queue": "0",
		"left":         "0",
		"member_time":  "1",
		"members":      "1",
		"query_queue":  "0",
		"query_time":   "1",
		"encrypted":    "false",
	}

	for key, val := range expected {
		v, ok := stats[key]
		if !ok {
			t.Fatalf("key not found in stats: %s", key)
		}
		if v != val {
			t.Fatalf("bad: %s = %s", key, val)
		}
	}
}

type CancelMergeDelegate struct {
	invoked bool
}

func (c *CancelMergeDelegate) NotifyMerge(members []*Member) (cancel bool) {
	log.Printf("Merge canceled")
	c.invoked = true
	return true
}

func TestSerf_Join_Cancel(t *testing.T) {
	s1Config := testConfig()
	merge1 := &CancelMergeDelegate{}
	s1Config.Merge = merge1

	s2Config := testConfig()
	merge2 := &CancelMergeDelegate{}
	s2Config.Merge = merge2

	s1, err := Create(s1Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	s2, err := Create(s2Config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	defer s1.Shutdown()
	defer s2.Shutdown()

	testutil.Yield()

	_, err = s1.Join([]string{s2Config.MemberlistConfig.BindAddr}, false)
	if err.Error() != "Merge canceled" {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if !merge1.invoked {
		t.Fatalf("should invoke")
	}
	if !merge2.invoked {
		t.Fatalf("should invoke")
	}
}
