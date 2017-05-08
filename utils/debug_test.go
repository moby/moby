package utils

import (
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
)

func TestEnableDebug(t *testing.T) {
	defer func() {
		os.Setenv("DEBUG", "")
		logrus.SetLevel(logrus.InfoLevel)
	}()
	EnableDebug()
	if os.Getenv("DEBUG") != "1" {
		t.Fatalf("expected DEBUG=1, got %s\n", os.Getenv("DEBUG"))
	}
	if logrus.GetLevel() != logrus.DebugLevel {
		t.Fatalf("expected log level %v, got %v\n", logrus.DebugLevel, logrus.GetLevel())
	}
}

func TestDisableDebug(t *testing.T) {
	DisableDebug()
	if os.Getenv("DEBUG") != "" {
		t.Fatalf("expected DEBUG=\"\", got %s\n", os.Getenv("DEBUG"))
	}
	if logrus.GetLevel() != logrus.InfoLevel {
		t.Fatalf("expected log level %v, got %v\n", logrus.InfoLevel, logrus.GetLevel())
	}
}

func TestDebugEnabled(t *testing.T) {
	EnableDebug()
	if !IsDebugEnabled() {
		t.Fatal("expected debug enabled, got false")
	}
	DisableDebug()
	if IsDebugEnabled() {
		t.Fatal("expected debug disabled, got true")
	}
}
