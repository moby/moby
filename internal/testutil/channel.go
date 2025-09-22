package testutil

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// ChannelGetOne reads one value from a channel or returns an error if the channel is closed or the timeout occurs.
func ChannelGetOne[T any](t *testing.T, ch <-chan T, timeout time.Duration) (*T, error) {
	select {
	case <-ch:
		v := <-ch
		return &v, nil
	case <-time.After(timeout):
		return nil, errors.New("timeout reading from channel")
	}

	return nil, fmt.Errorf("channel closed before reading value")
}
