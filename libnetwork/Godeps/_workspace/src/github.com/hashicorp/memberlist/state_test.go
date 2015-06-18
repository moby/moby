package memberlist

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

func HostMemberlist(host string, t *testing.T, f func(*Config)) *Memberlist {
	c := DefaultLANConfig()
	c.Name = host
	c.BindAddr = host
	if f != nil {
		f(c)
	}

	m, err := newMemberlist(c)
	if err != nil {
		t.Fatalf("failed to get memberlist: %s", err)
	}
	return m
}

func TestMemberList_Probe(t *testing.T) {
	addr1 := getBindAddr()
	addr2 := getBindAddr()
	m1 := HostMemberlist(addr1.String(), t, func(c *Config) {
		c.ProbeTimeout = time.Millisecond
		c.ProbeInterval = 10 * time.Millisecond
	})
	m2 := HostMemberlist(addr2.String(), t, nil)

	a1 := alive{
		Node:        addr1.String(),
		Addr:        []byte(addr1),
		Port:        uint16(m1.config.BindPort),
		Incarnation: 1,
	}
	m1.aliveNode(&a1, nil, true)
	a2 := alive{
		Node:        addr2.String(),
		Addr:        []byte(addr2),
		Port:        uint16(m2.config.BindPort),
		Incarnation: 1,
	}
	m1.aliveNode(&a2, nil, false)

	// should ping addr2
	m1.probe()

	// Should not be marked suspect
	n := m1.nodeMap[addr2.String()]
	if n.State != stateAlive {
		t.Fatalf("Expect node to be alive")
	}

	// Should increment seqno
	if m1.sequenceNum != 1 {
		t.Fatalf("bad seqno %v", m2.sequenceNum)
	}
}

func TestMemberList_ProbeNode_Suspect(t *testing.T) {
	addr1 := getBindAddr()
	addr2 := getBindAddr()
	addr3 := getBindAddr()
	addr4 := getBindAddr()
	ip1 := []byte(addr1)
	ip2 := []byte(addr2)
	ip3 := []byte(addr3)
	ip4 := []byte(addr4)

	m1 := HostMemberlist(addr1.String(), t, func(c *Config) {
		c.ProbeTimeout = time.Millisecond
		c.ProbeInterval = 10 * time.Millisecond
	})
	m2 := HostMemberlist(addr2.String(), t, nil)
	m3 := HostMemberlist(addr3.String(), t, nil)

	a1 := alive{Node: addr1.String(), Addr: ip1, Port: 7946, Incarnation: 1}
	m1.aliveNode(&a1, nil, true)
	a2 := alive{Node: addr2.String(), Addr: ip2, Port: 7946, Incarnation: 1}
	m1.aliveNode(&a2, nil, false)
	a3 := alive{Node: addr3.String(), Addr: ip3, Port: 7946, Incarnation: 1}
	m1.aliveNode(&a3, nil, false)
	a4 := alive{Node: addr4.String(), Addr: ip4, Port: 7946, Incarnation: 1}
	m1.aliveNode(&a4, nil, false)

	n := m1.nodeMap[addr4.String()]
	m1.probeNode(n)

	// Should be marked suspect
	if n.State != stateSuspect {
		t.Fatalf("Expect node to be suspect")
	}
	time.Sleep(10 * time.Millisecond)

	// Should increment seqno
	if m2.sequenceNum != 1 {
		t.Fatalf("bad seqno %v", m2.sequenceNum)
	}
	if m3.sequenceNum != 1 {
		t.Fatalf("bad seqno %v", m3.sequenceNum)
	}
}

func TestMemberList_ProbeNode(t *testing.T) {
	addr1 := getBindAddr()
	addr2 := getBindAddr()
	ip1 := []byte(addr1)
	ip2 := []byte(addr2)

	m1 := HostMemberlist(addr1.String(), t, func(c *Config) {
		c.ProbeTimeout = time.Millisecond
		c.ProbeInterval = 10 * time.Millisecond
	})
	m2 := HostMemberlist(addr2.String(), t, nil)

	a1 := alive{Node: addr1.String(), Addr: ip1, Port: 7946, Incarnation: 1}
	m1.aliveNode(&a1, nil, true)
	a2 := alive{Node: addr2.String(), Addr: ip2, Port: 7946, Incarnation: 1}
	m1.aliveNode(&a2, nil, false)

	n := m1.nodeMap[addr2.String()]
	m1.probeNode(n)

	// Should be marked suspect
	if n.State != stateAlive {
		t.Fatalf("Expect node to be alive")
	}

	// Should increment seqno
	if m1.sequenceNum != 1 {
		t.Fatalf("bad seqno %v", m2.sequenceNum)
	}
}

func TestMemberList_ResetNodes(t *testing.T) {
	m := GetMemberlist(t)
	a1 := alive{Node: "test1", Addr: []byte{127, 0, 0, 1}, Incarnation: 1}
	m.aliveNode(&a1, nil, false)
	a2 := alive{Node: "test2", Addr: []byte{127, 0, 0, 2}, Incarnation: 1}
	m.aliveNode(&a2, nil, false)
	a3 := alive{Node: "test3", Addr: []byte{127, 0, 0, 3}, Incarnation: 1}
	m.aliveNode(&a3, nil, false)
	d := dead{Node: "test2", Incarnation: 1}
	m.deadNode(&d)

	m.resetNodes()
	if len(m.nodes) != 2 {
		t.Fatalf("Bad length")
	}
	if _, ok := m.nodeMap["test2"]; ok {
		t.Fatalf("test2 should be unmapped")
	}
}

func TestMemberList_NextSeq(t *testing.T) {
	m := &Memberlist{}
	if m.nextSeqNo() != 1 {
		t.Fatalf("bad sequence no")
	}
	if m.nextSeqNo() != 2 {
		t.Fatalf("bad sequence no")
	}
}

func TestMemberList_SetAckChannel(t *testing.T) {
	m := &Memberlist{ackHandlers: make(map[uint32]*ackHandler)}

	ch := make(chan bool, 1)
	m.setAckChannel(0, ch, 10*time.Millisecond)

	if _, ok := m.ackHandlers[0]; !ok {
		t.Fatalf("missing handler")
	}
	time.Sleep(11 * time.Millisecond)

	if _, ok := m.ackHandlers[0]; ok {
		t.Fatalf("non-reaped handler")
	}
}

func TestMemberList_SetAckHandler(t *testing.T) {
	m := &Memberlist{ackHandlers: make(map[uint32]*ackHandler)}

	f := func() {}
	m.setAckHandler(0, f, 10*time.Millisecond)

	if _, ok := m.ackHandlers[0]; !ok {
		t.Fatalf("missing handler")
	}
	time.Sleep(11 * time.Millisecond)

	if _, ok := m.ackHandlers[0]; ok {
		t.Fatalf("non-reaped handler")
	}
}

func TestMemberList_InvokeAckHandler(t *testing.T) {
	m := &Memberlist{ackHandlers: make(map[uint32]*ackHandler)}

	// Does nothing
	m.invokeAckHandler(0)

	var b bool
	f := func() { b = true }
	m.setAckHandler(0, f, 10*time.Millisecond)

	// Should set b
	m.invokeAckHandler(0)
	if !b {
		t.Fatalf("b not set")
	}

	if _, ok := m.ackHandlers[0]; ok {
		t.Fatalf("non-reaped handler")
	}
}

func TestMemberList_InvokeAckHandler_Channel(t *testing.T) {
	m := &Memberlist{ackHandlers: make(map[uint32]*ackHandler)}

	// Does nothing
	m.invokeAckHandler(0)

	ch := make(chan bool, 1)
	m.setAckChannel(0, ch, 10*time.Millisecond)

	// Should send message
	m.invokeAckHandler(0)

	select {
	case v := <-ch:
		if v != true {
			t.Fatalf("Bad value")
		}
	default:
		t.Fatalf("message not sent")
	}

	if _, ok := m.ackHandlers[0]; ok {
		t.Fatalf("non-reaped handler")
	}
}

func TestMemberList_AliveNode_NewNode(t *testing.T) {
	ch := make(chan NodeEvent, 1)
	m := GetMemberlist(t)
	m.config.Events = &ChannelEventDelegate{ch}

	a := alive{Node: "test", Addr: []byte{127, 0, 0, 1}, Incarnation: 1}
	m.aliveNode(&a, nil, false)

	if len(m.nodes) != 1 {
		t.Fatalf("should add node")
	}

	state, ok := m.nodeMap["test"]
	if !ok {
		t.Fatalf("should map node")
	}

	if state.Incarnation != 1 {
		t.Fatalf("bad incarnation")
	}
	if state.State != stateAlive {
		t.Fatalf("bad state")
	}
	if time.Now().Sub(state.StateChange) > time.Second {
		t.Fatalf("bad change delta")
	}

	// Check for a join message
	select {
	case e := <-ch:
		if e.Node.Name != "test" {
			t.Fatalf("bad node name")
		}
	default:
		t.Fatalf("no join message")
	}

	// Check a broad cast is queued
	if m.broadcasts.NumQueued() != 1 {
		t.Fatalf("expected queued message")
	}
}

func TestMemberList_AliveNode_SuspectNode(t *testing.T) {
	ch := make(chan NodeEvent, 1)
	m := GetMemberlist(t)

	a := alive{Node: "test", Addr: []byte{127, 0, 0, 1}, Incarnation: 1}
	m.aliveNode(&a, nil, false)

	// Listen only after first join
	m.config.Events = &ChannelEventDelegate{ch}

	// Make suspect
	state := m.nodeMap["test"]
	state.State = stateSuspect
	state.StateChange = state.StateChange.Add(-time.Hour)

	// Old incarnation number, should not change
	m.aliveNode(&a, nil, false)
	if state.State != stateSuspect {
		t.Fatalf("update with old incarnation!")
	}

	// Should reset to alive now
	a.Incarnation = 2
	m.aliveNode(&a, nil, false)
	if state.State != stateAlive {
		t.Fatalf("no update with new incarnation!")
	}

	if time.Now().Sub(state.StateChange) > time.Second {
		t.Fatalf("bad change delta")
	}

	// Check for a no join message
	select {
	case <-ch:
		t.Fatalf("got bad join message")
	default:
	}

	// Check a broad cast is queued
	if m.broadcasts.NumQueued() != 1 {
		t.Fatalf("expected queued message")
	}
}

func TestMemberList_AliveNode_Idempotent(t *testing.T) {
	ch := make(chan NodeEvent, 1)
	m := GetMemberlist(t)

	a := alive{Node: "test", Addr: []byte{127, 0, 0, 1}, Incarnation: 1}
	m.aliveNode(&a, nil, false)

	// Listen only after first join
	m.config.Events = &ChannelEventDelegate{ch}

	// Make suspect
	state := m.nodeMap["test"]
	stateTime := state.StateChange

	// Should reset to alive now
	a.Incarnation = 2
	m.aliveNode(&a, nil, false)
	if state.State != stateAlive {
		t.Fatalf("non idempotent")
	}

	if stateTime != state.StateChange {
		t.Fatalf("should not change state")
	}

	// Check for a no join message
	select {
	case <-ch:
		t.Fatalf("got bad join message")
	default:
	}

	// Check a broad cast is queued
	if m.broadcasts.NumQueued() != 1 {
		t.Fatalf("expected only one queued message")
	}
}

// Serf Bug: GH-58, Meta data does not update
func TestMemberList_AliveNode_ChangeMeta(t *testing.T) {
	ch := make(chan NodeEvent, 1)
	m := GetMemberlist(t)

	a := alive{
		Node:        "test",
		Addr:        []byte{127, 0, 0, 1},
		Meta:        []byte("val1"),
		Incarnation: 1}
	m.aliveNode(&a, nil, false)

	// Listen only after first join
	m.config.Events = &ChannelEventDelegate{ch}

	// Make suspect
	state := m.nodeMap["test"]

	// Should reset to alive now
	a.Incarnation = 2
	a.Meta = []byte("val2")
	m.aliveNode(&a, nil, false)

	// Check updates
	if bytes.Compare(state.Meta, a.Meta) != 0 {
		t.Fatalf("meta did not update")
	}

	// Check for a NotifyUpdate
	select {
	case e := <-ch:
		if e.Event != NodeUpdate {
			t.Fatalf("bad event: %v", e)
		}
		if e.Node != &state.Node {
			t.Fatalf("bad event: %v", e)
		}
		if bytes.Compare(e.Node.Meta, a.Meta) != 0 {
			t.Fatalf("meta did not update")
		}
	default:
		t.Fatalf("missing event!")
	}

}

func TestMemberList_AliveNode_Refute(t *testing.T) {
	m := GetMemberlist(t)
	a := alive{Node: m.config.Name, Addr: []byte{127, 0, 0, 1}, Incarnation: 1}
	m.aliveNode(&a, nil, true)

	// Clear queue
	m.broadcasts.Reset()

	// Conflicting alive
	s := alive{
		Node:        m.config.Name,
		Addr:        []byte{127, 0, 0, 1},
		Incarnation: 2,
		Meta:        []byte("foo"),
	}
	m.aliveNode(&s, nil, false)

	state := m.nodeMap[m.config.Name]
	if state.State != stateAlive {
		t.Fatalf("should still be alive")
	}
	if state.Meta != nil {
		t.Fatalf("meta should still be nil")
	}

	// Check a broad cast is queued
	if num := m.broadcasts.NumQueued(); num != 1 {
		t.Fatalf("expected only one queued message: %d",
			num)
	}

	// Should be alive mesg
	if messageType(m.broadcasts.bcQueue[0].b.Message()[0]) != aliveMsg {
		t.Fatalf("expected queued alive msg")
	}
}

func TestMemberList_SuspectNode_NoNode(t *testing.T) {
	m := GetMemberlist(t)
	s := suspect{Node: "test", Incarnation: 1}
	m.suspectNode(&s)
	if len(m.nodes) != 0 {
		t.Fatalf("don't expect nodes")
	}
}

func TestMemberList_SuspectNode(t *testing.T) {
	m := GetMemberlist(t)
	m.config.ProbeInterval = time.Millisecond
	m.config.SuspicionMult = 1
	a := alive{Node: "test", Addr: []byte{127, 0, 0, 1}, Incarnation: 1}
	m.aliveNode(&a, nil, false)

	state := m.nodeMap["test"]
	state.StateChange = state.StateChange.Add(-time.Hour)

	s := suspect{Node: "test", Incarnation: 1}
	m.suspectNode(&s)

	if state.State != stateSuspect {
		t.Fatalf("Bad state")
	}

	change := state.StateChange
	if time.Now().Sub(change) > time.Second {
		t.Fatalf("bad change delta")
	}

	// Check a broad cast is queued
	if m.broadcasts.NumQueued() != 1 {
		t.Fatalf("expected only one queued message")
	}

	// Check its a suspect message
	if messageType(m.broadcasts.bcQueue[0].b.Message()[0]) != suspectMsg {
		t.Fatalf("expected queued suspect msg")
	}

	// Wait for the timeout
	time.Sleep(10 * time.Millisecond)

	if state.State != stateDead {
		t.Fatalf("Bad state")
	}

	if time.Now().Sub(state.StateChange) > time.Second {
		t.Fatalf("bad change delta")
	}
	if !state.StateChange.After(change) {
		t.Fatalf("should increment time")
	}

	// Check a broad cast is queued
	if m.broadcasts.NumQueued() != 1 {
		t.Fatalf("expected only one queued message")
	}

	// Check its a suspect message
	if messageType(m.broadcasts.bcQueue[0].b.Message()[0]) != deadMsg {
		t.Fatalf("expected queued dead msg")
	}
}

func TestMemberList_SuspectNode_DoubleSuspect(t *testing.T) {
	m := GetMemberlist(t)
	a := alive{Node: "test", Addr: []byte{127, 0, 0, 1}, Incarnation: 1}
	m.aliveNode(&a, nil, false)

	state := m.nodeMap["test"]
	state.StateChange = state.StateChange.Add(-time.Hour)

	s := suspect{Node: "test", Incarnation: 1}
	m.suspectNode(&s)

	if state.State != stateSuspect {
		t.Fatalf("Bad state")
	}

	change := state.StateChange
	if time.Now().Sub(change) > time.Second {
		t.Fatalf("bad change delta")
	}

	// clear the broadcast queue
	m.broadcasts.Reset()

	// Suspect again
	m.suspectNode(&s)

	if state.StateChange != change {
		t.Fatalf("unexpected state change")
	}

	// Check a broad cast is queued
	if m.broadcasts.NumQueued() != 0 {
		t.Fatalf("expected only one queued message")
	}

}

func TestMemberList_SuspectNode_OldSuspect(t *testing.T) {
	m := GetMemberlist(t)
	a := alive{Node: "test", Addr: []byte{127, 0, 0, 1}, Incarnation: 10}
	m.aliveNode(&a, nil, false)

	state := m.nodeMap["test"]
	state.StateChange = state.StateChange.Add(-time.Hour)

	// Clear queue
	m.broadcasts.Reset()

	s := suspect{Node: "test", Incarnation: 1}
	m.suspectNode(&s)

	if state.State != stateAlive {
		t.Fatalf("Bad state")
	}

	// Check a broad cast is queued
	if m.broadcasts.NumQueued() != 0 {
		t.Fatalf("expected only one queued message")
	}
}

func TestMemberList_SuspectNode_Refute(t *testing.T) {
	m := GetMemberlist(t)
	a := alive{Node: m.config.Name, Addr: []byte{127, 0, 0, 1}, Incarnation: 1}
	m.aliveNode(&a, nil, true)

	// Clear queue
	m.broadcasts.Reset()

	s := suspect{Node: m.config.Name, Incarnation: 1}
	m.suspectNode(&s)

	state := m.nodeMap[m.config.Name]
	if state.State != stateAlive {
		t.Fatalf("should still be alive")
	}

	// Check a broad cast is queued
	if m.broadcasts.NumQueued() != 1 {
		t.Fatalf("expected only one queued message")
	}

	// Should be alive mesg
	if messageType(m.broadcasts.bcQueue[0].b.Message()[0]) != aliveMsg {
		t.Fatalf("expected queued alive msg")
	}
}

func TestMemberList_DeadNode_NoNode(t *testing.T) {
	m := GetMemberlist(t)
	d := dead{Node: "test", Incarnation: 1}
	m.deadNode(&d)
	if len(m.nodes) != 0 {
		t.Fatalf("don't expect nodes")
	}
}

func TestMemberList_DeadNode(t *testing.T) {
	ch := make(chan NodeEvent, 1)
	m := GetMemberlist(t)
	m.config.Events = &ChannelEventDelegate{ch}
	a := alive{Node: "test", Addr: []byte{127, 0, 0, 1}, Incarnation: 1}
	m.aliveNode(&a, nil, false)

	// Read the join event
	<-ch

	state := m.nodeMap["test"]
	state.StateChange = state.StateChange.Add(-time.Hour)

	d := dead{Node: "test", Incarnation: 1}
	m.deadNode(&d)

	if state.State != stateDead {
		t.Fatalf("Bad state")
	}

	change := state.StateChange
	if time.Now().Sub(change) > time.Second {
		t.Fatalf("bad change delta")
	}

	select {
	case leave := <-ch:
		if leave.Event != NodeLeave || leave.Node.Name != "test" {
			t.Fatalf("bad node name")
		}
	default:
		t.Fatalf("no leave message")
	}

	// Check a broad cast is queued
	if m.broadcasts.NumQueued() != 1 {
		t.Fatalf("expected only one queued message")
	}

	// Check its a suspect message
	if messageType(m.broadcasts.bcQueue[0].b.Message()[0]) != deadMsg {
		t.Fatalf("expected queued dead msg")
	}
}

func TestMemberList_DeadNode_Double(t *testing.T) {
	ch := make(chan NodeEvent, 1)
	m := GetMemberlist(t)
	a := alive{Node: "test", Addr: []byte{127, 0, 0, 1}, Incarnation: 1}
	m.aliveNode(&a, nil, false)

	state := m.nodeMap["test"]
	state.StateChange = state.StateChange.Add(-time.Hour)

	d := dead{Node: "test", Incarnation: 1}
	m.deadNode(&d)

	// Clear queue
	m.broadcasts.Reset()

	// Notify after the first dead
	m.config.Events = &ChannelEventDelegate{ch}

	// Should do nothing
	d.Incarnation = 2
	m.deadNode(&d)

	select {
	case <-ch:
		t.Fatalf("should not get leave")
	default:
	}

	// Check a broad cast is queued
	if m.broadcasts.NumQueued() != 0 {
		t.Fatalf("expected only one queued message")
	}
}

func TestMemberList_DeadNode_OldDead(t *testing.T) {
	m := GetMemberlist(t)
	a := alive{Node: "test", Addr: []byte{127, 0, 0, 1}, Incarnation: 10}
	m.aliveNode(&a, nil, false)

	state := m.nodeMap["test"]
	state.StateChange = state.StateChange.Add(-time.Hour)

	d := dead{Node: "test", Incarnation: 1}
	m.deadNode(&d)

	if state.State != stateAlive {
		t.Fatalf("Bad state")
	}
}

func TestMemberList_DeadNode_AliveReplay(t *testing.T) {
	m := GetMemberlist(t)
	a := alive{Node: "test", Addr: []byte{127, 0, 0, 1}, Incarnation: 10}
	m.aliveNode(&a, nil, false)

	d := dead{Node: "test", Incarnation: 10}
	m.deadNode(&d)

	// Replay alive at same incarnation
	m.aliveNode(&a, nil, false)

	// Should remain dead
	state, ok := m.nodeMap["test"]
	if ok && state.State != stateDead {
		t.Fatalf("Bad state")
	}
}

func TestMemberList_DeadNode_Refute(t *testing.T) {
	m := GetMemberlist(t)
	a := alive{Node: m.config.Name, Addr: []byte{127, 0, 0, 1}, Incarnation: 1}
	m.aliveNode(&a, nil, true)

	// Clear queue
	m.broadcasts.Reset()

	d := dead{Node: m.config.Name, Incarnation: 1}
	m.deadNode(&d)

	state := m.nodeMap[m.config.Name]
	if state.State != stateAlive {
		t.Fatalf("should still be alive")
	}

	// Check a broad cast is queued
	if m.broadcasts.NumQueued() != 1 {
		t.Fatalf("expected only one queued message")
	}

	// Should be alive mesg
	if messageType(m.broadcasts.bcQueue[0].b.Message()[0]) != aliveMsg {
		t.Fatalf("expected queued alive msg")
	}
}

func TestMemberList_MergeState(t *testing.T) {
	m := GetMemberlist(t)
	a1 := alive{Node: "test1", Addr: []byte{127, 0, 0, 1}, Incarnation: 1}
	m.aliveNode(&a1, nil, false)
	a2 := alive{Node: "test2", Addr: []byte{127, 0, 0, 2}, Incarnation: 1}
	m.aliveNode(&a2, nil, false)
	a3 := alive{Node: "test3", Addr: []byte{127, 0, 0, 3}, Incarnation: 1}
	m.aliveNode(&a3, nil, false)

	s := suspect{Node: "test1", Incarnation: 1}
	m.suspectNode(&s)

	remote := []pushNodeState{
		pushNodeState{
			Name:        "test1",
			Addr:        []byte{127, 0, 0, 1},
			Incarnation: 2,
			State:       stateAlive,
		},
		pushNodeState{
			Name:        "test2",
			Addr:        []byte{127, 0, 0, 2},
			Incarnation: 1,
			State:       stateSuspect,
		},
		pushNodeState{
			Name:        "test3",
			Addr:        []byte{127, 0, 0, 3},
			Incarnation: 1,
			State:       stateDead,
		},
		pushNodeState{
			Name:        "test4",
			Addr:        []byte{127, 0, 0, 4},
			Incarnation: 2,
			State:       stateAlive,
		},
	}

	// Listen for changes
	eventCh := make(chan NodeEvent, 1)
	m.config.Events = &ChannelEventDelegate{eventCh}

	// Merge remote state
	m.mergeState(remote)

	// Check the states
	state := m.nodeMap["test1"]
	if state.State != stateAlive || state.Incarnation != 2 {
		t.Fatalf("Bad state %v", state)
	}

	state = m.nodeMap["test2"]
	if state.State != stateSuspect || state.Incarnation != 1 {
		t.Fatalf("Bad state %v", state)
	}

	state = m.nodeMap["test3"]
	if state.State != stateSuspect {
		t.Fatalf("Bad state %v", state)
	}

	state = m.nodeMap["test4"]
	if state.State != stateAlive || state.Incarnation != 2 {
		t.Fatalf("Bad state %v", state)
	}

	// Check the channels
	select {
	case e := <-eventCh:
		if e.Event != NodeJoin || e.Node.Name != "test4" {
			t.Fatalf("bad node %v", e)
		}
	default:
		t.Fatalf("Expect join")
	}

	select {
	case e := <-eventCh:
		t.Fatalf("Unexpect event: %v", e)
	default:
	}
}

func TestMemberlist_Gossip(t *testing.T) {
	ch := make(chan NodeEvent, 3)

	addr1 := getBindAddr()
	addr2 := getBindAddr()
	ip1 := []byte(addr1)
	ip2 := []byte(addr2)

	m1 := HostMemberlist(addr1.String(), t, func(c *Config) {
		c.GossipInterval = time.Millisecond
	})
	m2 := HostMemberlist(addr2.String(), t, func(c *Config) {
		c.Events = &ChannelEventDelegate{ch}
		c.GossipInterval = time.Millisecond
	})

	defer m1.Shutdown()
	defer m2.Shutdown()

	a1 := alive{Node: addr1.String(), Addr: ip1, Port: 7946, Incarnation: 1}
	m1.aliveNode(&a1, nil, true)
	a2 := alive{Node: addr2.String(), Addr: ip2, Port: 7946, Incarnation: 1}
	m1.aliveNode(&a2, nil, false)
	a3 := alive{Node: "172.0.0.1", Addr: []byte{172, 0, 0, 1}, Incarnation: 1}
	m1.aliveNode(&a3, nil, false)

	// Gossip should send all this to m2
	m1.gossip()

	for i := 0; i < 3; i++ {
		select {
		case <-ch:
		case <-time.After(50 * time.Millisecond):
			t.Fatalf("timeout")
		}
	}
}

func TestMemberlist_PushPull(t *testing.T) {
	addr1 := getBindAddr()
	addr2 := getBindAddr()
	ip1 := []byte(addr1)
	ip2 := []byte(addr2)

	ch := make(chan NodeEvent, 3)

	m1 := HostMemberlist(addr1.String(), t, func(c *Config) {
		c.GossipInterval = 10 * time.Second
		c.PushPullInterval = time.Millisecond
	})
	m2 := HostMemberlist(addr2.String(), t, func(c *Config) {
		c.GossipInterval = 10 * time.Second
		c.Events = &ChannelEventDelegate{ch}
	})

	defer m1.Shutdown()
	defer m2.Shutdown()

	a1 := alive{Node: addr1.String(), Addr: ip1, Port: 7946, Incarnation: 1}
	m1.aliveNode(&a1, nil, true)
	a2 := alive{Node: addr2.String(), Addr: ip2, Port: 7946, Incarnation: 1}
	m1.aliveNode(&a2, nil, false)

	// Gossip should send all this to m2
	m1.pushPull()

	for i := 0; i < 2; i++ {
		select {
		case <-ch:
		case <-time.After(10 * time.Millisecond):
			t.Fatalf("timeout")
		}
	}
}

func TestVerifyProtocol(t *testing.T) {
	cases := []struct {
		Anodes   [][3]uint8
		Bnodes   [][3]uint8
		expected bool
	}{
		// Both running identical everything
		{
			Anodes: [][3]uint8{
				{0, 0, 0},
			},
			Bnodes: [][3]uint8{
				{0, 0, 0},
			},
			expected: true,
		},

		// One can understand newer, but speaking same protocol
		{
			Anodes: [][3]uint8{
				{0, 0, 0},
			},
			Bnodes: [][3]uint8{
				{0, 1, 0},
			},
			expected: true,
		},

		// One is speaking outside the range
		{
			Anodes: [][3]uint8{
				{0, 0, 0},
			},
			Bnodes: [][3]uint8{
				{1, 1, 1},
			},
			expected: false,
		},

		// Transitively outside the range
		{
			Anodes: [][3]uint8{
				{0, 1, 0},
				{0, 2, 1},
			},
			Bnodes: [][3]uint8{
				{1, 3, 1},
			},
			expected: false,
		},

		// Multi-node
		{
			Anodes: [][3]uint8{
				{0, 3, 2},
				{0, 2, 0},
			},
			Bnodes: [][3]uint8{
				{0, 2, 1},
				{0, 5, 0},
			},
			expected: true,
		},
	}

	for _, tc := range cases {
		aCore := make([][6]uint8, len(tc.Anodes))
		aApp := make([][6]uint8, len(tc.Anodes))
		for i, n := range tc.Anodes {
			aCore[i] = [6]uint8{n[0], n[1], n[2], 0, 0, 0}
			aApp[i] = [6]uint8{0, 0, 0, n[0], n[1], n[2]}
		}

		bCore := make([][6]uint8, len(tc.Bnodes))
		bApp := make([][6]uint8, len(tc.Bnodes))
		for i, n := range tc.Bnodes {
			bCore[i] = [6]uint8{n[0], n[1], n[2], 0, 0, 0}
			bApp[i] = [6]uint8{0, 0, 0, n[0], n[1], n[2]}
		}

		// Test core protocol verification
		testVerifyProtocolSingle(t, aCore, bCore, tc.expected)
		testVerifyProtocolSingle(t, bCore, aCore, tc.expected)

		//  Test app protocol verification
		testVerifyProtocolSingle(t, aApp, bApp, tc.expected)
		testVerifyProtocolSingle(t, bApp, aApp, tc.expected)
	}
}

func testVerifyProtocolSingle(t *testing.T, A [][6]uint8, B [][6]uint8, expect bool) {
	m := GetMemberlist(t)
	defer m.Shutdown()

	m.nodes = make([]*nodeState, len(A))
	for i, n := range A {
		m.nodes[i] = &nodeState{
			Node: Node{
				PMin: n[0],
				PMax: n[1],
				PCur: n[2],
				DMin: n[3],
				DMax: n[4],
				DCur: n[5],
			},
		}
	}

	remote := make([]pushNodeState, len(B))
	for i, n := range B {
		remote[i] = pushNodeState{
			Name: fmt.Sprintf("node %d", i),
			Vsn:  []uint8{n[0], n[1], n[2], n[3], n[4], n[5]},
		}
	}

	err := m.verifyProtocol(remote)
	if (err == nil) != expect {
		t.Fatalf("bad:\nA: %v\nB: %v\nErr: %s", A, B, err)
	}
}
