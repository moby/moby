// +build ignore

package main

import (
	"testing"
	"time"

	"github.com/docker/docker/integration-cli/checker"
	"gotest.tools/assert"
	"gotest.tools/poll"
)

func pollCheck(t *testing.T, f interface{}, compare func(x interface{}) assert.BoolOrComparison) poll.Check

type eg_compareFunc func(...interface{}) checker.Compare

type waitAndAssertFunc func(t *testing.T, timeout time.Duration, ff, comparison interface{}, args ...interface{})

func before(
	waitAndAssert waitAndAssertFunc,
	t *testing.T,
	timeout time.Duration,
	f interface{},
	comparison interface{},
	args ...interface{}) {

	waitAndAssert(t, timeout, f, comparison, args...)
}

func after(
	waitAndAssert waitAndAssertFunc,
	t *testing.T,
	timeout time.Duration,
	f interface{},
	comparison interface{},
	args ...interface{}) {

	poll.WaitOn(t, pollCheck(t, f, comparison.(eg_compareFunc)(args...)), poll.WithTimeout(timeout))
}
