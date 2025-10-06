package filters

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestMarshalJSON(t *testing.T) {
	a := NewArgs(
		Arg("created", "today"),
		Arg("image.name", "ubuntu*"),
		Arg("image.name", "*untu"),
	)

	s, err := a.MarshalJSON()
	assert.Check(t, err)
	const expected = `{"created":{"today":true},"image.name":{"*untu":true,"ubuntu*":true}}`
	assert.Check(t, is.Equal(string(s), expected))
}

func TestMarshalJSONWithEmpty(t *testing.T) {
	s, err := json.Marshal(NewArgs())
	assert.Check(t, err)
	const expected = `{}`
	assert.Check(t, is.Equal(string(s), expected))
}

func TestToJSON(t *testing.T) {
	a := NewArgs(
		Arg("created", "today"),
		Arg("image.name", "ubuntu*"),
		Arg("image.name", "*untu"),
	)

	s, err := ToJSON(a)
	assert.Check(t, err)
	const expected = `{"created":{"today":true},"image.name":{"*untu":true,"ubuntu*":true}}`
	assert.Check(t, is.Equal(s, expected))
}

func TestAdd(t *testing.T) {
	f := NewArgs()
	f.Add("status", "running")
	v := f.fields["status"]
	assert.Check(t, is.DeepEqual(v, []string{"running"}))

	f.Add("status", "paused")
	v = f.fields["status"]
	assert.Check(t, is.Len(v, 2))
	assert.Check(t, is.Contains(v, "running"))
	assert.Check(t, is.Contains(v, "paused"))
}

func TestLen(t *testing.T) {
	f := NewArgs()
	assert.Check(t, is.Equal(f.Len(), 0))
	f.Add("status", "running")
	assert.Check(t, is.Equal(f.Len(), 1))
}
