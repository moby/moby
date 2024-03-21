package fakeclock

import (
	"errors"
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
)

type timeWatcher interface {
	timeUpdated(time.Time)
	shouldFire(time.Time) bool
	repeatable() bool
}

type FakeClock struct {
	now time.Time

	watchers map[timeWatcher]struct{}
	cond     *sync.Cond
}

func NewFakeClock(now time.Time) *FakeClock {
	return &FakeClock{
		now:      now,
		watchers: make(map[timeWatcher]struct{}),
		cond:     &sync.Cond{L: &sync.Mutex{}},
	}
}

func (clock *FakeClock) Since(t time.Time) time.Duration {
	return clock.Now().Sub(t)
}

func (clock *FakeClock) Now() time.Time {
	clock.cond.L.Lock()
	defer clock.cond.L.Unlock()

	return clock.now
}

func (clock *FakeClock) Increment(duration time.Duration) {
	clock.increment(duration, false, 0)
}

func (clock *FakeClock) IncrementBySeconds(seconds uint64) {
	clock.Increment(time.Duration(seconds) * time.Second)
}

func (clock *FakeClock) WaitForWatcherAndIncrement(duration time.Duration) {
	clock.WaitForNWatchersAndIncrement(duration, 1)
}

func (clock *FakeClock) WaitForNWatchersAndIncrement(duration time.Duration, numWatchers int) {
	clock.increment(duration, true, numWatchers)
}

func (clock *FakeClock) NewTimer(d time.Duration) clock.Timer {
	timer := newFakeTimer(clock, d, false)
	clock.addTimeWatcher(timer)

	return timer
}

func (clock *FakeClock) Sleep(d time.Duration) {
	<-clock.NewTimer(d).C()
}

func (clock *FakeClock) After(d time.Duration) <-chan time.Time {
	return clock.NewTimer(d).C()
}

func (clock *FakeClock) NewTicker(d time.Duration) clock.Ticker {
	if d <= 0 {
		panic(errors.New("duration must be greater than zero"))
	}

	timer := newFakeTimer(clock, d, true)
	clock.addTimeWatcher(timer)

	return newFakeTicker(timer)
}

func (clock *FakeClock) WatcherCount() int {
	clock.cond.L.Lock()
	defer clock.cond.L.Unlock()

	return len(clock.watchers)
}

func (clock *FakeClock) increment(duration time.Duration, waitForWatchers bool, numWatchers int) {
	clock.cond.L.Lock()

	for waitForWatchers && len(clock.watchers) < numWatchers {
		clock.cond.Wait()
	}

	now := clock.now.Add(duration)
	clock.now = now

	watchers := make([]timeWatcher, 0)
	newWatchers := map[timeWatcher]struct{}{}
	for w, _ := range clock.watchers {
		fire := w.shouldFire(now)
		if fire {
			watchers = append(watchers, w)
		}

		if !fire || w.repeatable() {
			newWatchers[w] = struct{}{}
		}
	}

	clock.watchers = newWatchers

	clock.cond.L.Unlock()

	for _, w := range watchers {
		w.timeUpdated(now)
	}
}

func (clock *FakeClock) addTimeWatcher(tw timeWatcher) {
	clock.cond.L.Lock()
	clock.watchers[tw] = struct{}{}
	clock.cond.L.Unlock()

	// force the timer to fire
	clock.Increment(0)

	clock.cond.Broadcast()
}

func (clock *FakeClock) removeTimeWatcher(tw timeWatcher) {
	clock.cond.L.Lock()
	delete(clock.watchers, tw)
	clock.cond.L.Unlock()
}
