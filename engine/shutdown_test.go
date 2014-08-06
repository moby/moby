package engine

import (
	"testing"
	"time"
)

func TestShutdownEmpty(t *testing.T) {
	eng := New()
	if eng.IsShutdown() {
		t.Fatalf("IsShutdown should be false")
	}
	eng.Shutdown()
	if !eng.IsShutdown() {
		t.Fatalf("IsShutdown should be true")
	}
}

func TestShutdownAfterRun(t *testing.T) {
	eng := New()
	var called bool
	eng.Register("foo", func(job *Job) Status {
		called = true
		return StatusOK
	})
	if err := eng.Job("foo").Run(); err != nil {
		t.Fatal(err)
	}
	eng.Shutdown()
	if err := eng.Job("foo").Run(); err == nil {
		t.Fatalf("%#v", *eng)
	}
}

// An approximate and racy, but better-than-nothing test that
//
func TestShutdownDuringRun(t *testing.T) {
	var (
		jobDelay     time.Duration = 500 * time.Millisecond
		jobDelayLow  time.Duration = 100 * time.Millisecond
		jobDelayHigh time.Duration = 700 * time.Millisecond
	)
	eng := New()
	var completed bool
	eng.Register("foo", func(job *Job) Status {
		time.Sleep(jobDelay)
		completed = true
		return StatusOK
	})
	go eng.Job("foo").Run()
	time.Sleep(50 * time.Millisecond)
	done := make(chan struct{})
	var startShutdown time.Time
	go func() {
		startShutdown = time.Now()
		eng.Shutdown()
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	if err := eng.Job("foo").Run(); err == nil {
		t.Fatalf("run on shutdown should fail: %#v", *eng)
	}
	<-done
	// Verify that Shutdown() blocks for roughly 500ms, instead
	// of returning almost instantly.
	//
	// We use >100ms to leave ample margin for race conditions between
	// goroutines. It's possible (but unlikely in reasonable testing
	// conditions), that this test will cause a false positive or false
	// negative. But it's probably better than not having any test
	// for the 99.999% of time where testing conditions are reasonable.
	if d := time.Since(startShutdown); d.Nanoseconds() < jobDelayLow.Nanoseconds() {
		t.Fatalf("shutdown did not block long enough: %v", d)
	} else if d.Nanoseconds() > jobDelayHigh.Nanoseconds() {
		t.Fatalf("shutdown blocked too long: %v", d)
	}
	if !completed {
		t.Fatalf("job did not complete")
	}
}
