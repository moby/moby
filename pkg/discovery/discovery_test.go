package discovery // import "github.com/docker/docker/pkg/discovery"

import (
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DiscoverySuite struct{}

var _ = check.Suite(&DiscoverySuite{})

func (s *DiscoverySuite) TestNewEntry(c *testing.T) {
	entry, err := NewEntry("127.0.0.1:2375")
	assert.Assert(c, err, checker.IsNil)
	assert.Equal(c, entry.Equals(&Entry{Host: "127.0.0.1", Port: "2375"}), true)
	assert.Equal(c, entry.String(), "127.0.0.1:2375")

	entry, err = NewEntry("[2001:db8:0:f101::2]:2375")
	assert.Assert(c, err, checker.IsNil)
	assert.Equal(c, entry.Equals(&Entry{Host: "2001:db8:0:f101::2", Port: "2375"}), true)
	assert.Equal(c, entry.String(), "[2001:db8:0:f101::2]:2375")

	_, err = NewEntry("127.0.0.1")
	assert.Assert(c, err, checker.NotNil)
}

func (s *DiscoverySuite) TestParse(c *testing.T) {
	scheme, uri := parse("127.0.0.1:2375")
	assert.Equal(c, scheme, "nodes")
	assert.Equal(c, uri, "127.0.0.1:2375")

	scheme, uri = parse("localhost:2375")
	assert.Equal(c, scheme, "nodes")
	assert.Equal(c, uri, "localhost:2375")

	scheme, uri = parse("scheme://127.0.0.1:2375")
	assert.Equal(c, scheme, "scheme")
	assert.Equal(c, uri, "127.0.0.1:2375")

	scheme, uri = parse("scheme://localhost:2375")
	assert.Equal(c, scheme, "scheme")
	assert.Equal(c, uri, "localhost:2375")

	scheme, uri = parse("")
	assert.Equal(c, scheme, "nodes")
	assert.Equal(c, uri, "")
}

func (s *DiscoverySuite) TestCreateEntries(c *testing.T) {
	entries, err := CreateEntries(nil)
	assert.DeepEqual(c, entries, Entries{})
	assert.Assert(c, err, checker.IsNil)

	entries, err = CreateEntries([]string{"127.0.0.1:2375", "127.0.0.2:2375", "[2001:db8:0:f101::2]:2375", ""})
	assert.Assert(c, err, checker.IsNil)
	expected := Entries{
		&Entry{Host: "127.0.0.1", Port: "2375"},
		&Entry{Host: "127.0.0.2", Port: "2375"},
		&Entry{Host: "2001:db8:0:f101::2", Port: "2375"},
	}
	assert.Equal(c, entries.Equals(expected), true)

	_, err = CreateEntries([]string{"127.0.0.1", "127.0.0.2"})
	assert.Assert(c, err, checker.NotNil)
}

func (s *DiscoverySuite) TestContainsEntry(c *testing.T) {
	entries, err := CreateEntries([]string{"127.0.0.1:2375", "127.0.0.2:2375", ""})
	assert.Assert(c, err, checker.IsNil)
	assert.Equal(c, entries.Contains(&Entry{Host: "127.0.0.1", Port: "2375"}), true)
	assert.Equal(c, entries.Contains(&Entry{Host: "127.0.0.3", Port: "2375"}), false)
}

func (s *DiscoverySuite) TestEntriesEquality(c *testing.T) {
	entries := Entries{
		&Entry{Host: "127.0.0.1", Port: "2375"},
		&Entry{Host: "127.0.0.2", Port: "2375"},
	}

	// Same
	assert.Assert(c, entries.Equals(Entries{
		&Entry{Host: "127.0.0.1", Port: "2375"},
		&Entry{Host: "127.0.0.2", Port: "2375"},
	}), checker.Equals, true)

	// Different size
	assert.Assert(c, entries.Equals(Entries{
		&Entry{Host: "127.0.0.1", Port: "2375"},
		&Entry{Host: "127.0.0.2", Port: "2375"},
		&Entry{Host: "127.0.0.3", Port: "2375"},
	}), checker.Equals, false)

	// Different content
	assert.Assert(c, entries.Equals(Entries{
		&Entry{Host: "127.0.0.1", Port: "2375"},
		&Entry{Host: "127.0.0.42", Port: "2375"},
	}), checker.Equals, false)

}

func (s *DiscoverySuite) TestEntriesDiff(c *testing.T) {
	entry1 := &Entry{Host: "1.1.1.1", Port: "1111"}
	entry2 := &Entry{Host: "2.2.2.2", Port: "2222"}
	entry3 := &Entry{Host: "3.3.3.3", Port: "3333"}
	entries := Entries{entry1, entry2}

	// No diff
	added, removed := entries.Diff(Entries{entry2, entry1})
	assert.Assert(c, added, checker.HasLen, 0)
	assert.Assert(c, removed, checker.HasLen, 0)

	// Add
	added, removed = entries.Diff(Entries{entry2, entry3, entry1})
	assert.Assert(c, added, checker.HasLen, 1)
	assert.Equal(c, added.Contains(entry3), true)
	assert.Assert(c, removed, checker.HasLen, 0)

	// Remove
	added, removed = entries.Diff(Entries{entry2})
	assert.Assert(c, added, checker.HasLen, 0)
	assert.Assert(c, removed, checker.HasLen, 1)
	assert.Equal(c, removed.Contains(entry1), true)

	// Add and remove
	added, removed = entries.Diff(Entries{entry1, entry3})
	assert.Assert(c, added, checker.HasLen, 1)
	assert.Equal(c, added.Contains(entry3), true)
	assert.Assert(c, removed, checker.HasLen, 1)
	assert.Equal(c, removed.Contains(entry2), true)
}
