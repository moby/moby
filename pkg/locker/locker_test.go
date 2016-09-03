package locker

import (
	"sync"
	"testing"
	"time"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestLockCounter(c *check.C) {
	l := &lockCtr{}
	l.inc()

	if l.waiters != 1 {
		c.Fatal("counter inc failed")
	}

	l.dec()
	if l.waiters != 0 {
		c.Fatal("counter dec failed")
	}
}

func (s *DockerSuite) TestLockerLock(c *check.C) {
	l := New()
	l.Lock("test")
	ctr := l.locks["test"]

	if ctr.count() != 0 {
		c.Fatalf("expected waiters to be 0, got :%d", ctr.waiters)
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
		c.Fatal("timed out waiting for lock waiters to be incremented")
	}

	select {
	case <-chDone:
		c.Fatal("lock should not have returned while it was still held")
	default:
	}

	if err := l.Unlock("test"); err != nil {
		c.Fatal(err)
	}

	select {
	case <-chDone:
	case <-time.After(3 * time.Second):
		c.Fatalf("lock should have completed")
	}

	if ctr.count() != 0 {
		c.Fatalf("expected waiters to be 0, got: %d", ctr.count())
	}
}

func (s *DockerSuite) TestLockerUnlock(c *check.C) {
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
		c.Fatalf("lock should not be blocked")
	}
}

func (s *DockerSuite) TestLockerConcurrency(c *check.C) {
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
		c.Fatal("timeout waiting for locks to complete")
	}

	// Since everything has unlocked this should not exist anymore
	if ctr, exists := l.locks["test"]; exists {
		c.Fatalf("lock should not exist: %v", ctr)
	}
}
