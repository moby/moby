package term // import "github.com/docker/docker/pkg/term"

import (
	"testing"

	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
)

func TestToBytes(t *testing.T) {
	codes, err := ToBytes("ctrl-a,a")
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual([]byte{1, 97}, codes))

	_, err = ToBytes("shift-z")
	assert.Check(t, is.ErrorContains(err, ""))

	codes, err = ToBytes("ctrl-@,ctrl-[,~,ctrl-o")
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual([]byte{0, 27, 126, 15}, codes))

	codes, err = ToBytes("DEL,+")
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual([]byte{127, 43}, codes))
}
