package term

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToBytes(t *testing.T) {
	codes, err := ToBytes("ctrl-a,a")
	require.NoError(t, err)
	assert.Equal(t, []byte{1, 97}, codes)

	_, err = ToBytes("shift-z")
	assert.Error(t, err)

	codes, err = ToBytes("ctrl-@,ctrl-[,~,ctrl-o")
	require.NoError(t, err)
	assert.Equal(t, []byte{0, 27, 126, 15}, codes)

	codes, err = ToBytes("DEL,+")
	require.NoError(t, err)
	assert.Equal(t, []byte{127, 43}, codes)
}
