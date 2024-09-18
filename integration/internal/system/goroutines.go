package system

import (
	"context"
	"time"

	"github.com/docker/docker/client"
	"gotest.tools/v3/poll"
)

// WaitForStableGoroutineCount polls the daemon Info API and returns the reported goroutine count
// after multiple calls return the same number.
func WaitForStableGoroutineCount(ctx context.Context, t poll.TestingT, apiClient client.SystemAPIClient, opts ...poll.SettingOp) int {
	var out int
	// Use a longish delay to make sure the goroutine count is actually stable.
	defaults := []poll.SettingOp{poll.WithTimeout(time.Minute), poll.WithDelay(time.Second)}
	opts = append(defaults, opts...)

	poll.WaitOn(t, StableGoroutineCount(ctx, apiClient, &out), opts...)
	return out
}

// StableGoroutineCount is a [poll.Check] that polls the daemon info API until the goroutine count is the same for 3 iterations.
func StableGoroutineCount(ctx context.Context, apiClient client.SystemAPIClient, count *int) poll.Check {
	var (
		numStable int
		nRoutines int
	)

	return func(t poll.LogT) poll.Result {
		n, err := getGoroutineNumber(ctx, apiClient)
		if err != nil {
			return poll.Error(err)
		}

		last := nRoutines

		if nRoutines == n {
			numStable++
		} else {
			numStable = 0
			nRoutines = n
		}

		if numStable > 3 {
			*count = n
			return poll.Success()
		}
		return poll.Continue("goroutine count is not stable: last %d, current %d, stable iters: %d", last, n, numStable)
	}
}

// CheckGoroutineCount returns a [poll.Check] that polls the daemon info API until the expected number of goroutines is hit.
func CheckGoroutineCount(ctx context.Context, apiClient client.SystemAPIClient, expected int) poll.Check {
	first := true
	return func(t poll.LogT) poll.Result {
		n, err := getGoroutineNumber(ctx, apiClient)
		if err != nil {
			return poll.Error(err)
		}
		if n > expected {
			if first {
				t.Log("Waiting for goroutines to stabilize")
				first = false
			}
			return poll.Continue("expected %d goroutines, got %d", expected, n)
		}
		return poll.Success()
	}
}

func getGoroutineNumber(ctx context.Context, apiClient client.SystemAPIClient) (int, error) {
	info, err := apiClient.Info(ctx)
	if err != nil {
		return 0, err
	}
	return info.NGoroutines, nil
}
