package debug

import (
	"os"
	"testing"

	"github.com/containerd/log"
)

func TestEnable(t *testing.T) {
	t.Setenv("DEBUG", "")
	l := log.GetLevel()
	t.Cleanup(func() { _ = log.SetLevel(l) })
	Enable()
	if debug := os.Getenv("DEBUG"); debug != "1" {
		t.Fatalf("expected DEBUG=1, got %s", debug)
	}
	if lvl := log.GetLevel(); lvl != log.DebugLevel {
		t.Fatalf("expected log level %v, got %v", log.DebugLevel, lvl)
	}
}

func TestDisable(t *testing.T) {
	t.Setenv("DEBUG", "1")
	l := log.GetLevel()
	t.Cleanup(func() { _ = log.SetLevel(l) })

	Disable()
	if debug := os.Getenv("DEBUG"); debug != "" {
		t.Fatalf(`expected DEBUG="", got %s`, debug)
	}
	if lvl := log.GetLevel(); lvl != log.InfoLevel {
		t.Fatalf("expected log level %v, got %v", log.InfoLevel, lvl)
	}
}

func TestEnabled(t *testing.T) {
	t.Setenv("DEBUG", "")
	l := log.GetLevel()
	t.Cleanup(func() { _ = log.SetLevel(l) })

	Enable()
	if !IsEnabled() {
		t.Fatal("expected debug enabled, got false")
	}
	Disable()
	if IsEnabled() {
		t.Fatal("expected debug disabled, got true")
	}
}
