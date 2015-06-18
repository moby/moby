package serf

import (
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/testutil"
	"testing"
)

func TestSerf_joinLeave_ltime(t *testing.T) {
	s1Config := testConfig()
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

	if s2.members[s1.config.NodeName].statusLTime != 1 {
		t.Fatalf("join time is not valid %d",
			s2.members[s1.config.NodeName].statusLTime)
	}

	if s2.clock.Time() <= s2.members[s1.config.NodeName].statusLTime {
		t.Fatalf("join should increment")
	}
	oldClock := s2.clock.Time()

	err = s1.Leave()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// s1 clock should exceed s2 due to leave
	if s2.clock.Time() <= oldClock {
		t.Fatalf("leave should increment (%d / %d)",
			s2.clock.Time(), oldClock)
	}
}

func TestSerf_join_pendingIntent(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	s.recentJoin[0] = nodeIntent{5, "test"}

	n := memberlist.Node{Name: "test",
		Addr: nil,
		Meta: []byte("test"),
	}

	s.handleNodeJoin(&n)

	mem := s.members["test"]
	if mem.statusLTime != 5 {
		t.Fatalf("bad join time")
	}
	if mem.Status != StatusAlive {
		t.Fatalf("bad status")
	}
}

func TestSerf_join_pendingIntents(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	s.recentJoin[0] = nodeIntent{5, "test"}
	s.recentLeave[0] = nodeIntent{6, "test"}

	n := memberlist.Node{Name: "test",
		Addr: nil,
		Meta: []byte("test"),
	}

	s.handleNodeJoin(&n)

	mem := s.members["test"]
	if mem.statusLTime != 6 {
		t.Fatalf("bad join time")
	}
	if mem.Status != StatusLeaving {
		t.Fatalf("bad status")
	}
}

func TestSerf_leaveIntent_bufferEarly(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	// Deliver a leave intent message early
	j := messageLeave{LTime: 10, Node: "test"}
	if !s.handleNodeLeaveIntent(&j) {
		t.Fatalf("should rebroadcast")
	}
	if s.handleNodeLeaveIntent(&j) {
		t.Fatalf("should not rebroadcast")
	}

	// Check that we buffered
	if s.recentLeaveIndex != 1 {
		t.Fatalf("bad index")
	}
	if s.recentLeave[0].Node != "test" || s.recentLeave[0].LTime != 10 {
		t.Fatalf("bad buffer")
	}
}

func TestSerf_leaveIntent_oldMessage(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	s.members["test"] = &memberState{
		Member: Member{
			Status: StatusAlive,
		},
		statusLTime: 12,
	}

	j := messageLeave{LTime: 10, Node: "test"}
	if s.handleNodeLeaveIntent(&j) {
		t.Fatalf("should not rebroadcast")
	}

	if s.recentLeaveIndex != 0 {
		t.Fatalf("bad index")
	}
}

func TestSerf_leaveIntent_newer(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	s.members["test"] = &memberState{
		Member: Member{
			Status: StatusAlive,
		},
		statusLTime: 12,
	}

	j := messageLeave{LTime: 14, Node: "test"}
	if !s.handleNodeLeaveIntent(&j) {
		t.Fatalf("should rebroadcast")
	}

	if s.recentLeaveIndex != 0 {
		t.Fatalf("bad index")
	}

	if s.members["test"].Status != StatusLeaving {
		t.Fatalf("should update status")
	}

	if s.clock.Time() != 15 {
		t.Fatalf("should update clock")
	}
}

func TestSerf_joinIntent_bufferEarly(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	// Deliver a join intent message early
	j := messageJoin{LTime: 10, Node: "test"}
	if !s.handleNodeJoinIntent(&j) {
		t.Fatalf("should rebroadcast")
	}
	if s.handleNodeJoinIntent(&j) {
		t.Fatalf("should not rebroadcast")
	}

	// Check that we buffered
	if s.recentJoinIndex != 1 {
		t.Fatalf("bad index")
	}
	if s.recentJoin[0].Node != "test" || s.recentJoin[0].LTime != 10 {
		t.Fatalf("bad buffer")
	}
}

func TestSerf_joinIntent_oldMessage(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	s.members["test"] = &memberState{
		statusLTime: 12,
	}

	j := messageJoin{LTime: 10, Node: "test"}
	if s.handleNodeJoinIntent(&j) {
		t.Fatalf("should not rebroadcast")
	}

	if s.recentJoinIndex != 0 {
		t.Fatalf("bad index")
	}
}

func TestSerf_joinIntent_newer(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	s.members["test"] = &memberState{
		statusLTime: 12,
	}

	// Deliver a join intent message early
	j := messageJoin{LTime: 14, Node: "test"}
	if !s.handleNodeJoinIntent(&j) {
		t.Fatalf("should rebroadcast")
	}

	if s.recentJoinIndex != 0 {
		t.Fatalf("bad index")
	}

	if s.members["test"].statusLTime != 14 {
		t.Fatalf("should update join time")
	}

	if s.clock.Time() != 15 {
		t.Fatalf("should update clock")
	}
}

func TestSerf_joinIntent_resetLeaving(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	s.members["test"] = &memberState{
		Member: Member{
			Status: StatusLeaving,
		},
		statusLTime: 12,
	}

	j := messageJoin{LTime: 14, Node: "test"}
	if !s.handleNodeJoinIntent(&j) {
		t.Fatalf("should rebroadcast")
	}

	if s.recentJoinIndex != 0 {
		t.Fatalf("bad index")
	}

	if s.members["test"].statusLTime != 14 {
		t.Fatalf("should update join time")
	}
	if s.members["test"].Status != StatusAlive {
		t.Fatalf("should update status")
	}

	if s.clock.Time() != 15 {
		t.Fatalf("should update clock")
	}
}

func TestSerf_userEvent_oldMessage(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	// increase the ltime artificially
	s.eventClock.Witness(LamportTime(c.EventBuffer + 1000))

	msg := messageUserEvent{
		LTime:   1,
		Name:    "old",
		Payload: nil,
	}
	if s.handleUserEvent(&msg) {
		t.Fatalf("should not rebroadcast")
	}
}

func TestSerf_userEvent_sameClock(t *testing.T) {
	eventCh := make(chan Event, 4)
	c := testConfig()
	c.EventCh = eventCh
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	msg := messageUserEvent{
		LTime:   1,
		Name:    "first",
		Payload: []byte("test"),
	}
	if !s.handleUserEvent(&msg) {
		t.Fatalf("should rebroadcast")
	}
	msg = messageUserEvent{
		LTime:   1,
		Name:    "first",
		Payload: []byte("newpayload"),
	}
	if !s.handleUserEvent(&msg) {
		t.Fatalf("should rebroadcast")
	}
	msg = messageUserEvent{
		LTime:   1,
		Name:    "second",
		Payload: []byte("other"),
	}
	if !s.handleUserEvent(&msg) {
		t.Fatalf("should rebroadcast")
	}

	testUserEvents(t, eventCh,
		[]string{"first", "first", "second"},
		[][]byte{[]byte("test"), []byte("newpayload"), []byte("other")})
}

func TestSerf_query_oldMessage(t *testing.T) {
	c := testConfig()
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	// increase the ltime artificially
	s.queryClock.Witness(LamportTime(c.QueryBuffer + 1000))

	msg := messageQuery{
		LTime:   1,
		Name:    "old",
		Payload: nil,
	}
	if s.handleQuery(&msg) {
		t.Fatalf("should not rebroadcast")
	}
}

func TestSerf_query_sameClock(t *testing.T) {
	eventCh := make(chan Event, 4)
	c := testConfig()
	c.EventCh = eventCh
	s, err := Create(c)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer s.Shutdown()

	msg := messageQuery{
		LTime:   1,
		ID:      1,
		Name:    "foo",
		Payload: []byte("test"),
	}
	if !s.handleQuery(&msg) {
		t.Fatalf("should rebroadcast")
	}
	if s.handleQuery(&msg) {
		t.Fatalf("should not rebroadcast")
	}
	msg = messageQuery{
		LTime:   1,
		ID:      2,
		Name:    "bar",
		Payload: []byte("newpayload"),
	}
	if !s.handleQuery(&msg) {
		t.Fatalf("should rebroadcast")
	}
	if s.handleQuery(&msg) {
		t.Fatalf("should not rebroadcast")
	}
	msg = messageQuery{
		LTime:   1,
		ID:      3,
		Name:    "baz",
		Payload: []byte("other"),
	}
	if !s.handleQuery(&msg) {
		t.Fatalf("should rebroadcast")
	}
	if s.handleQuery(&msg) {
		t.Fatalf("should not rebroadcast")
	}

	testQueryEvents(t, eventCh,
		[]string{"foo", "bar", "baz"},
		[][]byte{[]byte("test"), []byte("newpayload"), []byte("other")})
}
