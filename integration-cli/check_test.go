package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-check/check"
)

func Test(t *testing.T) { check.TestingT(t) }

type TimerSuite struct {
	start time.Time
}

func (s *TimerSuite) SetUpTest(c *check.C) {
	s.start = time.Now()
}

func (s *TimerSuite) TearDownTest(c *check.C) {
	fmt.Printf("%-60s%.2f\n", c.TestName(), time.Since(s.start).Seconds())
}

type DockerSuite struct {
	TimerSuite
}

func (s *DockerSuite) TearDownTest(c *check.C) {
	deleteAllContainers()
	s.TimerSuite.TearDownTest(c)
}

var _ = check.Suite(&DockerSuite{})
