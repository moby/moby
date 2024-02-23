package fakeclock

import (
	"sync"
	"time"
)

type fakeTimer struct {
	clock *FakeClock

	mutex          sync.Mutex
	completionTime time.Time
	channel        chan time.Time
	duration       time.Duration
	repeat         bool
}

func newFakeTimer(clock *FakeClock, d time.Duration, repeat bool) *fakeTimer {
	return &fakeTimer{
		clock:          clock,
		completionTime: clock.Now().Add(d),
		channel:        make(chan time.Time, 1),
		duration:       d,
		repeat:         repeat,
	}
}

func (ft *fakeTimer) C() <-chan time.Time {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()
	return ft.channel
}

func (ft *fakeTimer) reset(d time.Duration) bool {
	currentTime := ft.clock.Now()

	ft.mutex.Lock()
	active := !ft.completionTime.IsZero()
	ft.completionTime = currentTime.Add(d)
	ft.mutex.Unlock()
	return active
}

func (ft *fakeTimer) Reset(d time.Duration) bool {
	active := ft.reset(d)
	ft.clock.addTimeWatcher(ft)
	return active
}

func (ft *fakeTimer) Stop() bool {
	ft.mutex.Lock()
	active := !ft.completionTime.IsZero()
	ft.mutex.Unlock()

	ft.clock.removeTimeWatcher(ft)

	return active
}

func (ft *fakeTimer) shouldFire(now time.Time) bool {
	ft.mutex.Lock()
	defer ft.mutex.Unlock()

	if ft.completionTime.IsZero() {
		return false
	}

	return now.After(ft.completionTime) || now.Equal(ft.completionTime)
}

func (ft *fakeTimer) repeatable() bool {
	return ft.repeat
}

func (ft *fakeTimer) timeUpdated(now time.Time) {
	select {
	case ft.channel <- now:
	default:
		// drop on the floor. timers have a buffered channel anyway. according to
		// godoc of the `time' package a ticker can loose ticks in case of a slow
		// receiver
	}

	if ft.repeatable() {
		ft.reset(ft.duration)
	}
}
