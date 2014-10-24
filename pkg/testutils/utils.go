package testutils

import (
	"math/rand"
	"testing"
	"time"
)

const chars = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
	"~!@#$%^&*()-_+={}[]\\|<,>.?/\"';:` "

// Timeout calls f and waits for 100ms for it to complete.
// If it doesn't, it causes the tests to fail.
// t must be a valid testing context.
func Timeout(t *testing.T, f func()) {
	onTimeout := time.After(100 * time.Millisecond)
	onDone := make(chan bool)
	go func() {
		f()
		close(onDone)
	}()
	select {
	case <-onTimeout:
		t.Fatalf("timeout")
	case <-onDone:
	}
}

// RandomString returns random string of specified length
func RandomString(length int) string {
	res := make([]byte, length)
	for i := 0; i < length; i++ {
		res[i] = chars[rand.Intn(len(chars))]
	}
	return string(res)
}
