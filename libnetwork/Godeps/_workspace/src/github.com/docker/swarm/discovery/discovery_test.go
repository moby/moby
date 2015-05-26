package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEntry(t *testing.T) {
	entry, err := NewEntry("127.0.0.1:2375")
	assert.Equal(t, entry.Host, "127.0.0.1")
	assert.Equal(t, entry.Port, "2375")
	assert.NoError(t, err)

	_, err = NewEntry("127.0.0.1")
	assert.Error(t, err)
}

func TestParse(t *testing.T) {
	scheme, uri := parse("127.0.0.1:2375")
	assert.Equal(t, scheme, "nodes")
	assert.Equal(t, uri, "127.0.0.1:2375")

	scheme, uri = parse("localhost:2375")
	assert.Equal(t, scheme, "nodes")
	assert.Equal(t, uri, "localhost:2375")

	scheme, uri = parse("scheme://127.0.0.1:2375")
	assert.Equal(t, scheme, "scheme")
	assert.Equal(t, uri, "127.0.0.1:2375")

	scheme, uri = parse("scheme://localhost:2375")
	assert.Equal(t, scheme, "scheme")
	assert.Equal(t, uri, "localhost:2375")

	scheme, uri = parse("")
	assert.Equal(t, scheme, "nodes")
	assert.Equal(t, uri, "")
}

func TestCreateEntries(t *testing.T) {
	entries, err := CreateEntries(nil)
	assert.Equal(t, entries, []*Entry{})
	assert.NoError(t, err)

	entries, err = CreateEntries([]string{"127.0.0.1:2375", "127.0.0.2:2375", ""})
	assert.Equal(t, len(entries), 2)
	assert.Equal(t, entries[0].String(), "127.0.0.1:2375")
	assert.Equal(t, entries[1].String(), "127.0.0.2:2375")
	assert.NoError(t, err)

	_, err = CreateEntries([]string{"127.0.0.1", "127.0.0.2"})
	assert.Error(t, err)
}
