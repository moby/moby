package syslog

import (
	"errors"
	"testing"

	syslog "github.com/RackSec/srslog"
)

func TestLazy(t *testing.T) {
	numErrors := 2
	l := newLazyWriter(func() (*syslog.Writer, error) {
		// Pretend to have a few errors then succeed
		if numErrors > 0 {
			numErrors--
			return nil, errors.New("oh no I couldn't connect")
		}
		return nil, nil
	})

	_, err := l.GetOrConnect()
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = l.GetOrConnect()
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = l.GetOrConnect()
	if err != nil {
		t.Fatal("expected success")
	}
}

func TestClose(t *testing.T) {
	l := newLazyWriter(func() (*syslog.Writer, error) {
		// Pretend to always succeed
		return nil, nil
	})

	_, err := l.GetOrConnect()
	if err != nil {
		t.Fatal("expected success")
	}
	l.Close()
	_, err = l.GetOrConnect()
	if err == nil {
		t.Fatal("expected failure after closing")
	}
}
