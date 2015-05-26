package api

import (
	"log"
	"sync"
	"testing"
	"time"
)

func TestSemaphore_AcquireRelease(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	sema, err := c.SemaphorePrefix("test/semaphore", 2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Initial release should fail
	err = sema.Release()
	if err != ErrSemaphoreNotHeld {
		t.Fatalf("err: %v", err)
	}

	// Should work
	lockCh, err := sema.Acquire(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if lockCh == nil {
		t.Fatalf("not hold")
	}

	// Double lock should fail
	_, err = sema.Acquire(nil)
	if err != ErrSemaphoreHeld {
		t.Fatalf("err: %v", err)
	}

	// Should be held
	select {
	case <-lockCh:
		t.Fatalf("should be held")
	default:
	}

	// Initial release should work
	err = sema.Release()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Double unlock should fail
	err = sema.Release()
	if err != ErrSemaphoreNotHeld {
		t.Fatalf("err: %v", err)
	}

	// Should lose resource
	select {
	case <-lockCh:
	case <-time.After(time.Second):
		t.Fatalf("should not be held")
	}
}

func TestSemaphore_ForceInvalidate(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	sema, err := c.SemaphorePrefix("test/semaphore", 2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should work
	lockCh, err := sema.Acquire(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if lockCh == nil {
		t.Fatalf("not acquired")
	}
	defer sema.Release()

	go func() {
		// Nuke the session, simulator an operator invalidation
		// or a health check failure
		session := c.Session()
		session.Destroy(sema.lockSession, nil)
	}()

	// Should loose slot
	select {
	case <-lockCh:
	case <-time.After(time.Second):
		t.Fatalf("should not be locked")
	}
}

func TestSemaphore_DeleteKey(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	sema, err := c.SemaphorePrefix("test/semaphore", 2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should work
	lockCh, err := sema.Acquire(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if lockCh == nil {
		t.Fatalf("not locked")
	}
	defer sema.Release()

	go func() {
		// Nuke the key, simulate an operator intervention
		kv := c.KV()
		kv.DeleteTree("test/semaphore", nil)
	}()

	// Should loose leadership
	select {
	case <-lockCh:
	case <-time.After(time.Second):
		t.Fatalf("should not be locked")
	}
}

func TestSemaphore_Contend(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	wg := &sync.WaitGroup{}
	acquired := make([]bool, 4)
	for idx := range acquired {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sema, err := c.SemaphorePrefix("test/semaphore", 2)
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			// Should work eventually, will contend
			lockCh, err := sema.Acquire(nil)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if lockCh == nil {
				t.Fatalf("not locked")
			}
			defer sema.Release()
			log.Printf("Contender %d acquired", idx)

			// Set acquired and then leave
			acquired[idx] = true
		}(idx)
	}

	// Wait for termination
	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	// Wait for everybody to get a turn
	select {
	case <-doneCh:
	case <-time.After(3 * DefaultLockRetryTime):
		t.Fatalf("timeout")
	}

	for idx, did := range acquired {
		if !did {
			t.Fatalf("contender %d never acquired", idx)
		}
	}
}

func TestSemaphore_BadLimit(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	sema, err := c.SemaphorePrefix("test/semaphore", 0)
	if err == nil {
		t.Fatalf("should error")
	}

	sema, err = c.SemaphorePrefix("test/semaphore", 1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	_, err = sema.Acquire(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	sema2, err := c.SemaphorePrefix("test/semaphore", 2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	_, err = sema2.Acquire(nil)
	if err.Error() != "semaphore limit conflict (lock: 1, local: 2)" {
		t.Fatalf("err: %v", err)
	}
}

func TestSemaphore_Destroy(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	sema, err := c.SemaphorePrefix("test/semaphore", 2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	sema2, err := c.SemaphorePrefix("test/semaphore", 2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	_, err = sema.Acquire(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	_, err = sema2.Acquire(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Destroy should fail, still held
	if err := sema.Destroy(); err != ErrSemaphoreHeld {
		t.Fatalf("err: %v", err)
	}

	err = sema.Release()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Destroy should fail, still in use
	if err := sema.Destroy(); err != ErrSemaphoreInUse {
		t.Fatalf("err: %v", err)
	}

	err = sema2.Release()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Destroy should work
	if err := sema.Destroy(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Destroy should work
	if err := sema2.Destroy(); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestSemaphore_Conflict(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	lock, err := c.LockKey("test/sema/.lock")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should work
	leaderCh, err := lock.Lock(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if leaderCh == nil {
		t.Fatalf("not leader")
	}
	defer lock.Unlock()

	sema, err := c.SemaphorePrefix("test/sema/", 2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should conflict with lock
	_, err = sema.Acquire(nil)
	if err != ErrSemaphoreConflict {
		t.Fatalf("err: %v", err)
	}

	// Should conflict with lock
	err = sema.Destroy()
	if err != ErrSemaphoreConflict {
		t.Fatalf("err: %v", err)
	}
}
