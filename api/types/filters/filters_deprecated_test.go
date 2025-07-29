package filters

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestToParamWithVersion(t *testing.T) {
	a := NewArgs(
		Arg("created", "today"),
		Arg("image.name", "ubuntu*"),
		Arg("image.name", "*untu"),
	)

	str1, err := ToParamWithVersion("1.21", a)
	assert.Check(t, err)
	str2, err := ToParamWithVersion("1.22", a)
	assert.Check(t, err)
	if str1 != `{"created":["today"],"image.name":["*untu","ubuntu*"]}` &&
		str1 != `{"created":["today"],"image.name":["ubuntu*","*untu"]}` {
		t.Errorf("incorrectly marshaled the filters: %s", str1)
	}
	if str2 != `{"created":{"today":true},"image.name":{"*untu":true,"ubuntu*":true}}` &&
		str2 != `{"created":{"today":true},"image.name":{"ubuntu*":true,"*untu":true}}` {
		t.Errorf("incorrectly marshaled the filters: %s", str2)
	}
}
