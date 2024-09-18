// Package suite is a simplified version of testify's suite package which has unnecessary dependencies.
// Please remove this package whenever possible.
package suite

import (
	"context"
	"flag"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/docker/docker/testutil"
)

// TimeoutFlag is the flag to set a per-test timeout when running tests. Defaults to `-timeout`.
var TimeoutFlag = flag.Duration("timeout", 0, "DO NOT USE")

var typTestingT = reflect.TypeOf(new(testing.T))

// Run takes a testing suite and runs all of the tests attached to it.
func Run(ctx context.Context, t *testing.T, suite interface{}) {
	defer failOnPanic(t)

	ctx = testutil.StartSpan(ctx, t)
	suiteCtx := ctx

	suiteSetupDone := false

	defer func() {
		if suiteSetupDone {
			if tearDownAllSuite, ok := getTeardownAllSuite(suite); ok {
				tearDownAllSuite.TearDownSuite(suiteCtx, t)
			}
		}
	}()

	methodFinder := reflect.TypeOf(suite)
	for index := 0; index < methodFinder.NumMethod(); index++ {
		method := methodFinder.Method(index)
		if !methodFilter(method.Name, method.Type) {
			continue
		}
		t.Run(method.Name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			testutil.SetContext(t, ctx)
			t.Cleanup(func() {
				testutil.CleanupContext(t)
			})

			defer failOnPanic(t)

			if !suiteSetupDone {
				if setupAllSuite, ok := getSetupAllSuite(suite); ok {
					setupAllSuite.SetUpSuite(suiteCtx, t)
				}
				suiteSetupDone = true
			}

			if setupTestSuite, ok := getSetupTestSuite(suite); ok {
				setupTestSuite.SetUpTest(ctx, t)
			}
			defer func() {
				if tearDownTestSuite, ok := getTearDownTestSuite(suite); ok {
					tearDownTestSuite.TearDownTest(ctx, t)
				}
			}()

			method.Func.Call([]reflect.Value{reflect.ValueOf(suite), reflect.ValueOf(t)})
		})
	}
}

func getSetupAllSuite(suite interface{}) (SetupAllSuite, bool) {
	setupAllSuite, ok := suite.(SetupAllSuite)
	if ok {
		return setupAllSuite, ok
	}

	t := reflect.TypeOf(suite)
	for i := 0; i < t.NumMethod(); i++ {
		if t.Method(i).Name == "SetUpSuite" {
			panic("Wrong SetUpSuite signature")
		}
	}
	return nil, false
}

func getSetupTestSuite(suite interface{}) (SetupTestSuite, bool) {
	setupAllTest, ok := suite.(SetupTestSuite)
	if ok {
		return setupAllTest, ok
	}

	t := reflect.TypeOf(suite)
	for i := 0; i < t.NumMethod(); i++ {
		if t.Method(i).Name == "SetUpTest" {
			panic("Wrong SetUpTest signature")
		}
	}
	return nil, false
}

func getTearDownTestSuite(suite interface{}) (TearDownTestSuite, bool) {
	tearDownTest, ok := suite.(TearDownTestSuite)
	if ok {
		return tearDownTest, ok
	}

	t := reflect.TypeOf(suite)
	for i := 0; i < t.NumMethod(); i++ {
		if t.Method(i).Name == "TearDownTest" {
			panic("Wrong TearDownTest signature")
		}
	}
	return nil, false
}

func getTeardownAllSuite(suite interface{}) (TearDownAllSuite, bool) {
	tearDownAll, ok := suite.(TearDownAllSuite)
	if ok {
		return tearDownAll, ok
	}

	t := reflect.TypeOf(suite)
	for i := 0; i < t.NumMethod(); i++ {
		if t.Method(i).Name == "TearDownSuite" {
			panic("Wrong TearDownSuite signature")
		}
	}
	return nil, false
}

func failOnPanic(t *testing.T) {
	r := recover()
	if r != nil {
		t.Errorf("test suite panicked: %v\n%s", r, debug.Stack())
		t.FailNow()
	}
}

func methodFilter(name string, typ reflect.Type) bool {
	return strings.HasPrefix(name, "Test") && typ.NumIn() == 2 && typ.In(1) == typTestingT // 2 params: method receiver and *testing.T
}
