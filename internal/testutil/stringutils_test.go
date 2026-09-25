package testutil

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestRandomAlphaOnlyString(t *testing.T) {
	expectedLength := 20
	s := RandomAlpha(expectedLength)
	assert.Check(t, is.Equal(len(s), expectedLength))
}
