package serf

import (
	"testing"
)

func TestLamportClock(t *testing.T) {
	l := &LamportClock{}

	if l.Time() != 0 {
		t.Fatalf("bad time value")
	}

	if l.Increment() != 1 {
		t.Fatalf("bad time value")
	}

	if l.Time() != 1 {
		t.Fatalf("bad time value")
	}

	l.Witness(41)

	if l.Time() != 42 {
		t.Fatalf("bad time value")
	}

	l.Witness(41)

	if l.Time() != 42 {
		t.Fatalf("bad time value")
	}

	l.Witness(30)

	if l.Time() != 42 {
		t.Fatalf("bad time value")
	}
}
