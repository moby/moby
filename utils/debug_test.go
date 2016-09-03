package utils

import (
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestEnableDebug(c *check.C) {
	defer func() {
		os.Setenv("DEBUG", "")
		logrus.SetLevel(logrus.InfoLevel)
	}()
	EnableDebug()
	if os.Getenv("DEBUG") != "1" {
		c.Fatalf("expected DEBUG=1, got %s\n", os.Getenv("DEBUG"))
	}
	if logrus.GetLevel() != logrus.DebugLevel {
		c.Fatalf("expected log level %v, got %v\n", logrus.DebugLevel, logrus.GetLevel())
	}
}

func (s *DockerSuite) TestDisableDebug(c *check.C) {
	DisableDebug()
	if os.Getenv("DEBUG") != "" {
		c.Fatalf("expected DEBUG=\"\", got %s\n", os.Getenv("DEBUG"))
	}
	if logrus.GetLevel() != logrus.InfoLevel {
		c.Fatalf("expected log level %v, got %v\n", logrus.InfoLevel, logrus.GetLevel())
	}
}

func (s *DockerSuite) TestDebugEnabled(c *check.C) {
	EnableDebug()
	if !IsDebugEnabled() {
		c.Fatal("expected debug enabled, got false")
	}
	DisableDebug()
	if IsDebugEnabled() {
		c.Fatal("expected debug disabled, got true")
	}
}
