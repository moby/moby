package checker

import (
	"reflect"
	"testing"

	"github.com/go-check/check"
)

func Test(t *testing.T) {
	check.TestingT(t)
}

func init() {
	check.Suite(&CheckersS{})
}

type CheckersS struct{}

var _ = check.Suite(&CheckersS{})

func testInfo(c *check.C, checker check.Checker, name string, paramNames []string) {
	info := checker.Info()
	if info.Name != name {
		c.Fatalf("Got name %s, expected %s", info.Name, name)
	}
	if !reflect.DeepEqual(info.Params, paramNames) {
		c.Fatalf("Got param names %#v, expected %#v", info.Params, paramNames)
	}
}

func testCheck(c *check.C, checker check.Checker, expectedResult bool, expectedError string, params ...interface{}) ([]interface{}, []string) {
	info := checker.Info()
	if len(params) != len(info.Params) {
		c.Fatalf("unexpected param count in test; expected %d got %d", len(info.Params), len(params))
	}
	names := append([]string{}, info.Params...)
	result, error := checker.Check(params, names)
	if result != expectedResult || error != expectedError {
		c.Fatalf("%s.Check(%#v) returned (%#v, %#v) rather than (%#v, %#v)",
			info.Name, params, result, error, expectedResult, expectedError)
	}
	return params, names
}

func (s *CheckersS) TestContains(c *check.C) {
	testInfo(c, Contains, "Contains", []string{"value", "substring"})

	testCheck(c, Contains, true, "", "abcd", "bc")
	testCheck(c, Contains, false, "", "abcd", "efg")
	testCheck(c, Contains, false, "", "", "bc")
	testCheck(c, Contains, true, "", "abcd", "")
	testCheck(c, Contains, true, "", "", "")

	testCheck(c, Contains, false, "Obtained value is not a string and has no .String()", 12, "1")
	testCheck(c, Contains, false, "Substring must be a string", "", 1)
}
