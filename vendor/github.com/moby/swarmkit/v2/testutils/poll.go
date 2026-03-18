package testutils

import (
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"github.com/pkg/errors"
)

// PollFuncWithTimeout is used to periodically execute a check function, it
// returns error after timeout.
func PollFuncWithTimeout(clockSource *fakeclock.FakeClock, f func() error, timeout time.Duration) error {
	if f() == nil {
		return nil
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for i := 0; ; i++ {
		if i%5 == 0 && clockSource != nil {
			clockSource.Increment(time.Second)
		}
		err := f()
		if err == nil {
			return nil
		}
		select {
		case <-timer.C:
			return errors.Wrap(err, "polling failed")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// PollFunc is like PollFuncWithTimeout with timeout=10s.
func PollFunc(clockSource *fakeclock.FakeClock, f func() error) error {
	return PollFuncWithTimeout(clockSource, f, 10*time.Second)
}
