package templates // import "github.com/docker/docker/daemon/logger/templates"

import (
	"bytes"
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestNewParse(t *testing.T) {
	tm, err := NewParse("foo", "this is a {{ . }}")
	assert.Check(t, err)

	var b bytes.Buffer
	assert.Check(t, tm.Execute(&b, "string"))
	want := "this is a string"
	assert.Check(t, is.Equal(want, b.String()))
}
