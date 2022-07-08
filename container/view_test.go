package container // import "github.com/docker/docker/container"

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stringid"
	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

var root string

func TestMain(m *testing.M) {
	var err error
	root, err = os.MkdirTemp("", "docker-container-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)

	os.Exit(m.Run())
}

func newContainer(t *testing.T) *Container {
	var (
		id    = uuid.New().String()
		cRoot = filepath.Join(root, id)
	)
	if err := os.MkdirAll(cRoot, 0755); err != nil {
		t.Fatal(err)
	}
	c := NewBaseContainer(id, cRoot)
	c.HostConfig = &containertypes.HostConfig{}
	return c
}

func TestViewSaveDelete(t *testing.T) {
	db, err := NewViewDB()
	if err != nil {
		t.Fatal(err)
	}
	c := newContainer(t)
	if err := c.CheckpointTo(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(c); err != nil {
		t.Fatal(err)
	}
}

func TestViewAll(t *testing.T) {
	var (
		db, _ = NewViewDB()
		one   = newContainer(t)
		two   = newContainer(t)
	)
	one.Pid = 10
	if err := one.CheckpointTo(db); err != nil {
		t.Fatal(err)
	}
	two.Pid = 20
	if err := two.CheckpointTo(db); err != nil {
		t.Fatal(err)
	}

	all, err := db.Snapshot().All()
	if err != nil {
		t.Fatal(err)
	}
	if l := len(all); l != 2 {
		t.Fatalf("expected 2 items, got %d", l)
	}
	byID := make(map[string]Snapshot)
	for i := range all {
		byID[all[i].ID] = all[i]
	}
	if s, ok := byID[one.ID]; !ok || s.Pid != 10 {
		t.Fatalf("expected something different with for id=%s: %v", one.ID, s)
	}
	if s, ok := byID[two.ID]; !ok || s.Pid != 20 {
		t.Fatalf("expected something different with for id=%s: %v", two.ID, s)
	}
}

func TestViewGet(t *testing.T) {
	var (
		db, _ = NewViewDB()
		one   = newContainer(t)
	)
	one.ImageID = "some-image-123"
	if err := one.CheckpointTo(db); err != nil {
		t.Fatal(err)
	}
	s, err := db.Snapshot().Get(one.ID)
	if err != nil {
		t.Fatal(err)
	}
	if s == nil || s.ImageID != "some-image-123" {
		t.Fatalf("expected ImageID=some-image-123. Got: %v", s)
	}
}

func TestNames(t *testing.T) {
	db, err := NewViewDB()
	if err != nil {
		t.Fatal(err)
	}
	assert.Check(t, db.ReserveName("name1", "containerid1"))
	assert.Check(t, db.ReserveName("name1", "containerid1")) // idempotent
	assert.Check(t, db.ReserveName("name2", "containerid2"))
	assert.Check(t, is.Error(db.ReserveName("name2", "containerid3"), ErrNameReserved.Error()))

	// Releasing a name allows the name to point to something else later.
	assert.Check(t, db.ReleaseName("name2"))
	assert.Check(t, db.ReserveName("name2", "containerid3"))

	view := db.Snapshot()

	id, err := view.GetID("name1")
	assert.Check(t, err)
	assert.Check(t, is.Equal("containerid1", id))

	id, err = view.GetID("name2")
	assert.Check(t, err)
	assert.Check(t, is.Equal("containerid3", id))

	_, err = view.GetID("notreserved")
	assert.Check(t, is.Error(err, ErrNameNotReserved.Error()))

	// Releasing and re-reserving a name doesn't affect the snapshot.
	assert.Check(t, db.ReleaseName("name2"))
	assert.Check(t, db.ReserveName("name2", "containerid4"))

	id, err = view.GetID("name1")
	assert.Check(t, err)
	assert.Check(t, is.Equal("containerid1", id))

	id, err = view.GetID("name2")
	assert.Check(t, err)
	assert.Check(t, is.Equal("containerid3", id))

	// GetAllNames
	assert.Check(t, is.DeepEqual(map[string][]string{"containerid1": {"name1"}, "containerid3": {"name2"}}, view.GetAllNames()))

	assert.Check(t, db.ReserveName("name3", "containerid1"))
	assert.Check(t, db.ReserveName("name4", "containerid1"))

	view = db.Snapshot()
	assert.Check(t, is.DeepEqual(map[string][]string{"containerid1": {"name1", "name3", "name4"}, "containerid4": {"name2"}}, view.GetAllNames()))

	// Release containerid1's names with Delete even though no container exists
	assert.Check(t, db.Delete(&Container{ID: "containerid1"}))

	// Reusing one of those names should work
	assert.Check(t, db.ReserveName("name1", "containerid4"))
	view = db.Snapshot()
	assert.Check(t, is.DeepEqual(map[string][]string{"containerid4": {"name1", "name2"}}, view.GetAllNames()))
}

// Test case for GitHub issue 35920
func TestViewWithHealthCheck(t *testing.T) {
	var (
		db, _ = NewViewDB()
		one   = newContainer(t)
	)
	one.Health = &Health{
		Health: types.Health{
			Status: "starting",
		},
	}
	if err := one.CheckpointTo(db); err != nil {
		t.Fatal(err)
	}
	s, err := db.Snapshot().Get(one.ID)
	if err != nil {
		t.Fatal(err)
	}
	if s == nil || s.Health != "starting" {
		t.Fatalf("expected Health=starting. Got: %+v", s)
	}
}

func TestTruncIndex(t *testing.T) {
	db, err := NewViewDB()
	if err != nil {
		t.Fatal(err)
	}

	// Get on an empty index
	if _, err := db.GetByPrefix("foobar"); err == nil {
		t.Fatal("Get on an empty index should return an error")
	}

	id := "99b36c2c326ccc11e726eee6ee78a0baf166ef96"
	// Add an id
	if err := db.Save(&Container{ID: id}); err != nil {
		t.Fatal(err)
	}

	type testacase struct {
		name           string
		input          string
		expectedResult string
		expectError    bool
	}

	for _, tc := range []testacase{
		{
			name:        "Get a non-existing id",
			input:       "abracadabra",
			expectError: true,
		},
		{
			name:           "Get an empty id",
			input:          "",
			expectedResult: "",
			expectError:    true,
		},
		{
			name:           "Get the exact id",
			input:          id,
			expectedResult: id,
			expectError:    false,
		},
		{
			name:           "The first letter should match",
			input:          id[:1],
			expectedResult: id,
			expectError:    false,
		},
		{
			name:           "The first half should match",
			input:          id[:len(id)/2],
			expectedResult: id,
			expectError:    false,
		},
		{
			name:        "The second half should NOT match",
			input:       id[len(id)/2:],
			expectError: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertIndexGet(t, db, tc.input, tc.expectedResult, tc.expectError)
		})
	}

	id2 := id[:6] + "blabla"
	// Add an id
	if err := db.Save(&Container{ID: id2}); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []testacase{
		{
			name:           "id should work",
			input:          id,
			expectedResult: id,
			expectError:    false,
		},
		{
			name:           "id2 should work",
			input:          id2,
			expectedResult: id2,
			expectError:    false,
		},
		{
			name:           "6 characters should conflict",
			input:          id[:6],
			expectedResult: "",
			expectError:    true,
		},
		{
			name:           "4 characters should conflict",
			input:          id[:4],
			expectedResult: "",
			expectError:    true,
		},
		{
			name:           "1 character should conflict",
			input:          id[:6],
			expectedResult: "",
			expectError:    true,
		},
		{
			name:           "7 characters of id should not conflict",
			input:          id[:7],
			expectedResult: id,
			expectError:    false,
		},
		{
			name:           "7 characters of id2 should not conflict",
			input:          id2[:7],
			expectedResult: id2,
			expectError:    false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertIndexGet(t, db, tc.input, tc.expectedResult, tc.expectError)
		})
	}

	// Deleting id2 should remove conflicts
	if err := db.Delete(&Container{ID: id2}); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []testacase{
		{
			name:           "id2 should no longer work",
			input:          id2,
			expectedResult: "",
			expectError:    true,
		},
		{
			name:           "7 characters id2 should no longer work",
			input:          id2[:7],
			expectedResult: "",
			expectError:    true,
		},
		{
			name:           "11 characters id2 should no longer work",
			input:          id2[:11],
			expectedResult: "",
			expectError:    true,
		},
		{
			name:           "conflicts between id[:6] and id2 should be gone",
			input:          id[:6],
			expectedResult: id,
			expectError:    false,
		},
		{
			name:           "conflicts between id[:4] and id2 should be gone",
			input:          id[:4],
			expectedResult: id,
			expectError:    false,
		},
		{
			name:           "conflicts between id[:1] and id2 should be gone",
			input:          id[:1],
			expectedResult: id,
			expectError:    false,
		},
		{
			name:           "non-conflicting 7 character substrings should still not conflict",
			input:          id[:7],
			expectedResult: id,
			expectError:    false,
		},
		{
			name:           "non-conflicting 15 character substrings should still not conflict",
			input:          id[:15],
			expectedResult: id,
			expectError:    false,
		},
		{
			name:           "non-conflicting substrings should still not conflict",
			input:          id,
			expectedResult: id,
			expectError:    false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertIndexGet(t, db, tc.input, tc.expectedResult, tc.expectError)
		})
	}
}

func assertIndexGet(t *testing.T, snapshot ViewDB, input, expectedResult string, expectError bool) {
	if result, err := snapshot.GetByPrefix(input); err != nil && !expectError {
		t.Fatalf("Unexpected error getting '%s': %s", input, err)
	} else if err == nil && expectError {
		t.Fatalf("Getting '%s' should return an error, not '%s'", input, result)
	} else if result != expectedResult {
		t.Fatalf("Getting '%s' returned '%s' instead of '%s'", input, result, expectedResult)
	}
}

func BenchmarkDBAdd100(b *testing.B) {
	var testSet []string
	for i := 0; i < 100; i++ {
		testSet = append(testSet, stringid.GenerateRandomID())
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db, err := NewViewDB()
		if err != nil {
			b.Fatal(err)
		}
		for _, id := range testSet {
			if err := db.Save(&Container{ID: id}); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkDBGetByPrefix100(b *testing.B) {
	var testSet []string
	var testKeys []string
	for i := 0; i < 100; i++ {
		testSet = append(testSet, stringid.GenerateRandomID())
	}
	db, err := NewViewDB()
	if err != nil {
		b.Fatal(err)
	}
	for _, id := range testSet {
		if err := db.Save(&Container{ID: id}); err != nil {
			b.Fatal(err)
		}
		l := rand.Intn(12) + 12
		testKeys = append(testKeys, id[:l])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, id := range testKeys {
			if res, err := db.GetByPrefix(id); err != nil {
				b.Fatal(res, err)
			}
		}
	}
}

func BenchmarkDBGetByPrefix250(b *testing.B) {
	var testSet []string
	var testKeys []string
	for i := 0; i < 250; i++ {
		testSet = append(testSet, stringid.GenerateRandomID())
	}
	db, err := NewViewDB()
	if err != nil {
		b.Fatal(err)
	}
	for _, id := range testSet {
		if err := db.Save(&Container{ID: id}); err != nil {
			b.Fatal(err)
		}
		l := rand.Intn(12) + 12
		testKeys = append(testKeys, id[:l])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, id := range testKeys {
			if res, err := db.GetByPrefix(id); err != nil {
				b.Fatal(res, err)
			}
		}
	}
}

func BenchmarkDBGetByPrefix500(b *testing.B) {
	var testSet []string
	var testKeys []string
	for i := 0; i < 500; i++ {
		testSet = append(testSet, stringid.GenerateRandomID())
	}
	db, err := NewViewDB()
	if err != nil {
		b.Fatal(err)
	}
	for _, id := range testSet {
		if err := db.Save(&Container{ID: id}); err != nil {
			b.Fatal(err)
		}
		l := rand.Intn(12) + 12
		testKeys = append(testKeys, id[:l])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, id := range testKeys {
			if res, err := db.GetByPrefix(id); err != nil {
				b.Fatal(res, err)
			}
		}
	}
}
