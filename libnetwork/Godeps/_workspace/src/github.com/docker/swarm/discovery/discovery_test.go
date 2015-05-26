package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEntry(t *testing.T) {
	entry, err := NewEntry("127.0.0.1:2375")
	assert.NoError(t, err)
	assert.True(t, entry.Equals(&Entry{Host: "127.0.0.1", Port: "2375"}))
	assert.Equal(t, entry.String(), "127.0.0.1:2375")

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
	assert.Equal(t, entries, Entries{})
	assert.NoError(t, err)

	entries, err = CreateEntries([]string{"127.0.0.1:2375", "127.0.0.2:2375", ""})
	assert.NoError(t, err)
	expected := Entries{
		&Entry{Host: "127.0.0.1", Port: "2375"},
		&Entry{Host: "127.0.0.2", Port: "2375"},
	}
	assert.True(t, entries.Equals(expected))

	_, err = CreateEntries([]string{"127.0.0.1", "127.0.0.2"})
	assert.Error(t, err)
}

func TestContainsEntry(t *testing.T) {
	entries, err := CreateEntries([]string{"127.0.0.1:2375", "127.0.0.2:2375", ""})
	assert.NoError(t, err)
	assert.True(t, entries.Contains(&Entry{Host: "127.0.0.1", Port: "2375"}))
	assert.False(t, entries.Contains(&Entry{Host: "127.0.0.3", Port: "2375"}))
}

func TestEntriesEquality(t *testing.T) {
	entries := Entries{
		&Entry{Host: "127.0.0.1", Port: "2375"},
		&Entry{Host: "127.0.0.2", Port: "2375"},
	}

	// Same
	assert.True(t, entries.Equals(Entries{
		&Entry{Host: "127.0.0.1", Port: "2375"},
		&Entry{Host: "127.0.0.2", Port: "2375"},
	}))

	// Different size
	assert.False(t, entries.Equals(Entries{
		&Entry{Host: "127.0.0.1", Port: "2375"},
		&Entry{Host: "127.0.0.2", Port: "2375"},
		&Entry{Host: "127.0.0.3", Port: "2375"},
	}))

	// Different content
	assert.False(t, entries.Equals(Entries{
		&Entry{Host: "127.0.0.1", Port: "2375"},
		&Entry{Host: "127.0.0.42", Port: "2375"},
	}))
}

func TestEntriesDiff(t *testing.T) {
	entry1 := &Entry{Host: "1.1.1.1", Port: "1111"}
	entry2 := &Entry{Host: "2.2.2.2", Port: "2222"}
	entry3 := &Entry{Host: "3.3.3.3", Port: "3333"}
	entries := Entries{entry1, entry2}

	// No diff
	added, removed := entries.Diff(Entries{entry2, entry1})
	assert.Empty(t, added)
	assert.Empty(t, removed)

	// Add
	added, removed = entries.Diff(Entries{entry2, entry3, entry1})
	assert.Len(t, added, 1)
	assert.True(t, added.Contains(entry3))
	assert.Empty(t, removed)

	// Remove
	added, removed = entries.Diff(Entries{entry2})
	assert.Empty(t, added)
	assert.Len(t, removed, 1)
	assert.True(t, removed.Contains(entry1))

	// Add and remove
	added, removed = entries.Diff(Entries{entry1, entry3})
	assert.Len(t, added, 1)
	assert.True(t, added.Contains(entry3))
	assert.Len(t, removed, 1)
	assert.True(t, removed.Contains(entry2))
}
