package main

import (
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/utils"
)

func TestClientDebugEnabled(t *testing.T) {
	defer utils.DisableDebug()

	clientFlags.Common.FlagSet.Parse([]string{"-D"})
	clientFlags.PostParse()

	if os.Getenv("DEBUG") != "1" {
		t.Fatal("expected debug enabled, got false")
	}
	if logrus.GetLevel() != logrus.DebugLevel {
		t.Fatalf("expected logrus debug level, got %v", logrus.GetLevel())
	}
}
