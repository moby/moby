package locker

import (
	"testing"
	"time"
)

func TestLockCounter(t *testing.T) {
	l := &lockCtr{}
	l.inc()

	if l.waiters != 1 {
		t.Fatal("counter inc failed")
	}

	l.dec()
	if l.waiters != 0 {
		t.Fatal("counter dec failed")
	}
}

func TestLockerLock(t *testing.T) {
	l := New()
	l.Lock("test")
	ctr := l.locks["test"]

	if ctr.count() != 0 {
		t.Fatalf("expected waiters to be 0, got :%d", ctr.waiters)
	}

	chDone := make(chan struct{})
	go func() {
		l.Lock("test")
		close(chDone)
	}()

	chWaiting := make(chan struct{})
	go func() {
		for range time.Tick(1 * time.Millisecond) {
			if ctr.count() == 1 {
				close(chWaiting)
				break
			}
		}
	}()

	select {
	case <-chWaiting:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for lock waiters to be incremented")
	}

	select {
	case <-chDone:
		t.Fatal("lock should not have returned while it was still held")
	default:
	}

	if err := l.Unlock("test"); err != nil {
		t.Fatal(err)
	}

	select {
	case <-chDone:
	case <-time.After(3 * time.Second):
		t.Fatalf("lock should have completed")
	}

	if ctr.count() != 0 {
		t.Fatalf("expected waiters to be 0, got: %d", ctr.count())
	}
}

func TestLockerUnlock(t *testing.T) {
	l := New()

	l.Lock("test")
	l.Unlock("test")

	chDone := make(chan struct{})
	go func() {
		l.Lock("test")
		close(chDone)
	}()

	select {
	case <-chDone:
	case <-time.After(3 * time.Second):
		t.Fatalf("lock should not be blocked")
	}
}
