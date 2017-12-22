package term // import "github.com/docker/docker/pkg/term"

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEscapeProxyRead(t *testing.T) {
	escapeKeys, _ := ToBytes("DEL")
	keys, _ := ToBytes("a,b,c,+")
	reader := NewEscapeProxy(bytes.NewReader(keys), escapeKeys)
	buf := make([]byte, len(keys))
	nr, err := reader.Read(buf)
	require.NoError(t, err)
	require.EqualValues(t, nr, len(keys), fmt.Sprintf("nr %d should be equal to the number of %d", nr, len(keys)))
	require.Equal(t, keys, buf, "keys & the read buffer should be equal")

	keys, _ = ToBytes("")
	reader = NewEscapeProxy(bytes.NewReader(keys), escapeKeys)
	buf = make([]byte, len(keys))
	nr, err = reader.Read(buf)
	require.Error(t, err, "Should throw error when no keys are to read")
	require.EqualValues(t, nr, 0, "nr should be zero")
	assert.Len(t, keys, 0)
	assert.Len(t, buf, 0)

	escapeKeys, _ = ToBytes("ctrl-x,ctrl-@")
	keys, _ = ToBytes("DEL")
	reader = NewEscapeProxy(bytes.NewReader(keys), escapeKeys)
	buf = make([]byte, len(keys))
	nr, err = reader.Read(buf)
	require.NoError(t, err)
	require.EqualValues(t, nr, 1, fmt.Sprintf("nr %d should be equal to the number of 1", nr))
	require.Equal(t, keys, buf, "keys & the read buffer should be equal")

	escapeKeys, _ = ToBytes("ctrl-c")
	keys, _ = ToBytes("ctrl-c")
	reader = NewEscapeProxy(bytes.NewReader(keys), escapeKeys)
	buf = make([]byte, len(keys))
	nr, err = reader.Read(buf)
	require.EqualError(t, err, "read escape sequence")
	require.EqualValues(t, nr, 0, "nr should be equal to 0")
	require.Equal(t, keys, buf, "keys & the read buffer should be equal")

	escapeKeys, _ = ToBytes("ctrl-c,ctrl-z")
	keys, _ = ToBytes("ctrl-c,ctrl-z")
	reader = NewEscapeProxy(bytes.NewReader(keys), escapeKeys)
	buf = make([]byte, 1)
	nr, err = reader.Read(buf)
	require.NoError(t, err)
	require.EqualValues(t, nr, 0, "nr should be equal to 0")
	require.Equal(t, keys[0:1], buf, "keys & the read buffer should be equal")
	nr, err = reader.Read(buf)
	require.EqualError(t, err, "read escape sequence")
	require.EqualValues(t, nr, 0, "nr should be equal to 0")
	require.Equal(t, keys[1:], buf, "keys & the read buffer should be equal")

	escapeKeys, _ = ToBytes("ctrl-c,ctrl-z")
	keys, _ = ToBytes("ctrl-c,DEL,+")
	reader = NewEscapeProxy(bytes.NewReader(keys), escapeKeys)
	buf = make([]byte, 1)
	nr, err = reader.Read(buf)
	require.NoError(t, err)
	require.EqualValues(t, nr, 0, "nr should be equal to 0")
	require.Equal(t, keys[0:1], buf, "keys & the read buffer should be equal")
	buf = make([]byte, len(keys))
	nr, err = reader.Read(buf)
	require.NoError(t, err)
	require.EqualValues(t, nr, len(keys), fmt.Sprintf("nr should be equal to %d", len(keys)))
	require.Equal(t, keys, buf, "keys & the read buffer should be equal")

	escapeKeys, _ = ToBytes("ctrl-c,ctrl-z")
	keys, _ = ToBytes("ctrl-c,DEL")
	reader = NewEscapeProxy(bytes.NewReader(keys), escapeKeys)
	buf = make([]byte, 1)
	nr, err = reader.Read(buf)
	require.NoError(t, err)
	require.EqualValues(t, nr, 0, "nr should be equal to 0")
	require.Equal(t, keys[0:1], buf, "keys & the read buffer should be equal")
	buf = make([]byte, len(keys))
	nr, err = reader.Read(buf)
	require.NoError(t, err)
	require.EqualValues(t, nr, len(keys), fmt.Sprintf("nr should be equal to %d", len(keys)))
	require.Equal(t, keys, buf, "keys & the read buffer should be equal")
}
