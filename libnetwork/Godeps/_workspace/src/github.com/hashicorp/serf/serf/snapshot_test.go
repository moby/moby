package serf

import (
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestSnapshoter(t *testing.T) {
	td, err := ioutil.TempDir("", "serf")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(td)

	clock := new(LamportClock)
	outCh := make(chan Event, 64)
	stopCh := make(chan struct{})
	logger := log.New(os.Stderr, "", log.LstdFlags)
	inCh, snap, err := NewSnapshotter(td+"snap", snapshotSizeLimit, false,
		logger, clock, outCh, stopCh)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Write some user events
	ue := UserEvent{
		LTime: 42,
		Name:  "bar",
	}
	inCh <- ue

	// Write some queries
	q := &Query{
		LTime: 50,
		Name:  "uptime",
	}
	inCh <- q

	// Write some member events
	clock.Witness(100)
	meJoin := MemberEvent{
		Type: EventMemberJoin,
		Members: []Member{
			Member{
				Name: "foo",
				Addr: []byte{127, 0, 0, 1},
				Port: 5000,
			},
		},
	}
	meFail := MemberEvent{
		Type: EventMemberFailed,
		Members: []Member{
			Member{
				Name: "foo",
				Addr: []byte{127, 0, 0, 1},
				Port: 5000,
			},
		},
	}
	inCh <- meJoin
	inCh <- meFail
	inCh <- meJoin

	// Check these get passed through
	select {
	case e := <-outCh:
		if !reflect.DeepEqual(e, ue) {
			t.Fatalf("expected user event: %#v", e)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout")
	}

	select {
	case e := <-outCh:
		if !reflect.DeepEqual(e, q) {
			t.Fatalf("expected query event: %#v", e)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout")
	}

	select {
	case e := <-outCh:
		if !reflect.DeepEqual(e, meJoin) {
			t.Fatalf("expected member event: %#v", e)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout")
	}

	select {
	case e := <-outCh:
		if !reflect.DeepEqual(e, meFail) {
			t.Fatalf("expected member event: %#v", e)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout")
	}

	select {
	case e := <-outCh:
		if !reflect.DeepEqual(e, meJoin) {
			t.Fatalf("expected member event: %#v", e)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeout")
	}

	// Close the snapshoter
	close(stopCh)
	snap.Wait()

	// Open the snapshoter
	stopCh = make(chan struct{})
	_, snap, err = NewSnapshotter(td+"snap", snapshotSizeLimit, false,
		logger, clock, outCh, stopCh)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the values
	if snap.LastClock() != 100 {
		t.Fatalf("bad clock %d", snap.LastClock())
	}
	if snap.LastEventClock() != 42 {
		t.Fatalf("bad clock %d", snap.LastEventClock())
	}
	if snap.LastQueryClock() != 50 {
		t.Fatalf("bad clock %d", snap.LastQueryClock())
	}

	prev := snap.AliveNodes()
	if len(prev) != 1 {
		t.Fatalf("expected alive: %#v", prev)
	}
	if prev[0].Name != "foo" {
		t.Fatalf("bad name: %#v", prev[0])
	}
	if prev[0].Addr != "127.0.0.1:5000" {
		t.Fatalf("bad addr: %#v", prev[0])
	}
}

func TestSnapshoter_forceCompact(t *testing.T) {
	td, err := ioutil.TempDir("", "serf")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(td)

	clock := new(LamportClock)
	stopCh := make(chan struct{})
	logger := log.New(os.Stderr, "", log.LstdFlags)

	// Create a very low limit
	inCh, snap, err := NewSnapshotter(td+"snap", 1024, false,
		logger, clock, nil, stopCh)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Write lots of user events
	for i := 0; i < 1024; i++ {
		ue := UserEvent{
			LTime: LamportTime(i),
		}
		inCh <- ue
	}

	// Write lots of queries
	for i := 0; i < 1024; i++ {
		q := &Query{
			LTime: LamportTime(i),
		}
		inCh <- q
	}

	// Wait for drain
	for len(inCh) > 0 {
		time.Sleep(20 * time.Millisecond)
	}

	// Close the snapshoter
	close(stopCh)
	snap.Wait()

	// Open the snapshoter
	stopCh = make(chan struct{})
	_, snap, err = NewSnapshotter(td+"snap", snapshotSizeLimit, false,
		logger, clock, nil, stopCh)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the values
	if snap.LastEventClock() != 1023 {
		t.Fatalf("bad clock %d", snap.LastEventClock())
	}

	if snap.LastQueryClock() != 1023 {
		t.Fatalf("bad clock %d", snap.LastQueryClock())
	}

	close(stopCh)
	snap.Wait()
}

func TestSnapshoter_leave(t *testing.T) {
	td, err := ioutil.TempDir("", "serf")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(td)

	clock := new(LamportClock)
	stopCh := make(chan struct{})
	logger := log.New(os.Stderr, "", log.LstdFlags)
	inCh, snap, err := NewSnapshotter(td+"snap", snapshotSizeLimit, false,
		logger, clock, nil, stopCh)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Write a user event
	ue := UserEvent{
		LTime: 42,
		Name:  "bar",
	}
	inCh <- ue

	// Write a query
	q := &Query{
		LTime: 50,
		Name:  "uptime",
	}
	inCh <- q

	// Write some member events
	clock.Witness(100)
	meJoin := MemberEvent{
		Type: EventMemberJoin,
		Members: []Member{
			Member{
				Name: "foo",
				Addr: []byte{127, 0, 0, 1},
				Port: 5000,
			},
		},
	}
	inCh <- meJoin

	// Wait for drain
	for len(inCh) > 0 {
		time.Sleep(20 * time.Millisecond)
	}

	// Leave the cluster!
	snap.Leave()

	// Close the snapshoter
	close(stopCh)
	snap.Wait()

	// Open the snapshoter
	stopCh = make(chan struct{})
	_, snap, err = NewSnapshotter(td+"snap", snapshotSizeLimit, false,
		logger, clock, nil, stopCh)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the values
	if snap.LastClock() != 0 {
		t.Fatalf("bad clock %d", snap.LastClock())
	}
	if snap.LastEventClock() != 0 {
		t.Fatalf("bad clock %d", snap.LastEventClock())
	}
	if snap.LastQueryClock() != 0 {
		t.Fatalf("bad clock %d", snap.LastQueryClock())
	}

	prev := snap.AliveNodes()
	if len(prev) != 0 {
		t.Fatalf("expected none alive: %#v", prev)
	}
}

func TestSnapshoter_leave_rejoin(t *testing.T) {
	td, err := ioutil.TempDir("", "serf")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(td)

	clock := new(LamportClock)
	stopCh := make(chan struct{})
	logger := log.New(os.Stderr, "", log.LstdFlags)
	inCh, snap, err := NewSnapshotter(td+"snap", snapshotSizeLimit, true,
		logger, clock, nil, stopCh)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Write a user event
	ue := UserEvent{
		LTime: 42,
		Name:  "bar",
	}
	inCh <- ue

	// Write a query
	q := &Query{
		LTime: 50,
		Name:  "uptime",
	}
	inCh <- q

	// Write some member events
	clock.Witness(100)
	meJoin := MemberEvent{
		Type: EventMemberJoin,
		Members: []Member{
			Member{
				Name: "foo",
				Addr: []byte{127, 0, 0, 1},
				Port: 5000,
			},
		},
	}
	inCh <- meJoin

	// Wait for drain
	for len(inCh) > 0 {
		time.Sleep(20 * time.Millisecond)
	}

	// Leave the cluster!
	snap.Leave()

	// Close the snapshoter
	close(stopCh)
	snap.Wait()

	// Open the snapshoter
	stopCh = make(chan struct{})
	_, snap, err = NewSnapshotter(td+"snap", snapshotSizeLimit, true,
		logger, clock, nil, stopCh)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the values
	if snap.LastClock() != 100 {
		t.Fatalf("bad clock %d", snap.LastClock())
	}
	if snap.LastEventClock() != 42 {
		t.Fatalf("bad clock %d", snap.LastEventClock())
	}
	if snap.LastQueryClock() != 50 {
		t.Fatalf("bad clock %d", snap.LastQueryClock())
	}

	prev := snap.AliveNodes()
	if len(prev) == 0 {
		t.Fatalf("expected alive: %#v", prev)
	}
}
