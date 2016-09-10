package locker

import (
	"fmt"
	"sync"
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

func TestLockerConcurrency(t *testing.T) {
	l := New()

	var wg sync.WaitGroup
	for i := 0; i <= 10000; i++ {
		wg.Add(1)
		go func() {
			l.Lock("test")
			// if there is a concurrency issue, will very likely panic here
			l.Unlock("test")
			wg.Done()
		}()
	}

	chDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(chDone)
	}()

	select {
	case <-chDone:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for locks to complete")
	}

	// Since everything has unlocked this should not exist anymore
	if ctr, exists := l.locks["test"]; exists {
		t.Fatalf("lock should not exist: %v", ctr)
	}
}

func TestLockerCancelWithError(t *testing.T) {
	// 1. (Outside) Lock
	// 2. (Outside) CancelWithError
	// 3. -> GoInside
	// 4.    (Inside)  Lock (later, it should get an error, not the lock)
	// 5. (Outside) Unlock
	// 6.    Now, the inside Lock gets the lock chance,
	//       but it checks locks[name].err, find error, so it Unlock
	//       Then, the locks[name] is deleted
	//    <- (Inside) Return error, lock fail
	// 7. (Outside) Lock this name again
	//    (this time, it is a new lock, so it won't return error)

	l := New()
	testErr := fmt.Errorf("test error")

	if err := l.Lock("test"); err != nil {
		t.Fatalf("lock's error should be nil before CancelWithError")
	}
	l.CancelWithError("test", testErr)

	chErr := make(chan error)
	goInside := make(chan struct{})
	defer close(chErr)
	go func() {
		<-goInside
		err := l.Lock("test")
		chErr <- err
	}()

	goInside <- struct{}{}
	l.Unlock("test")

	select {
	case err := <-chErr:
		if err != testErr {
			t.Fatalf("lock's error is '%v', should be '%v'", err, testErr)
		}
		// Since lock has an error, the lock should not be Locked
		if ctr, exists := l.locks["test"]; exists {
			t.Fatalf("lock should not exist: %v", ctr)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("lock should not be blocked")
	}

	if err := l.Lock("test"); err != nil {
		t.Fatalf("lock's error should be nil because it is a new lock")
	}
	l.Unlock("test")
}
