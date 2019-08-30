// Package suite is a simplified version of testify's suite package which has unnecessary dependencies.
// Please remove this package whenever possible.
package suite

import (
	"flag"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
)

// TimeoutFlag is the flag to set a per-test timeout when running tests. Defaults to `-timeout`.
var TimeoutFlag = flag.Duration("timeout", 0, "DO NOT USE")

var typTestingT = reflect.TypeOf(new(testing.T))

// Run takes a testing suite and runs all of the tests attached to it.
func Run(t *testing.T, suite interface{}) {
	defer failOnPanic(t)

	suiteSetupDone := false

	methodFinder := reflect.TypeOf(suite)
	suiteName := methodFinder.Elem().Name()
	for index := 0; index < methodFinder.NumMethod(); index++ {
		method := methodFinder.Method(index)
		if !methodFilter(method.Name, method.Type) {
			continue
		}
		if !suiteSetupDone {
			if setupAllSuite, ok := suite.(SetupAllSuite); ok {
				setupAllSuite.SetUpSuite(t)
			}
			defer func() {
				if tearDownAllSuite, ok := suite.(TearDownAllSuite); ok {
					tearDownAllSuite.TearDownSuite(t)
				}
			}()
			suiteSetupDone = true
		}
		t.Run(suiteName+"/"+method.Name, func(t *testing.T) {
			defer failOnPanic(t)

			if setupTestSuite, ok := suite.(SetupTestSuite); ok {
				setupTestSuite.SetUpTest(t)
			}
			defer func() {
				if tearDownTestSuite, ok := suite.(TearDownTestSuite); ok {
					tearDownTestSuite.TearDownTest(t)
				}
			}()

			method.Func.Call([]reflect.Value{reflect.ValueOf(suite), reflect.ValueOf(t)})
		})
	}
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
