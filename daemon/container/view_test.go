package container

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/google/uuid"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/internal/stringid"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func newContainer(t *testing.T, root string) *Container {
	t.Helper()
	id := uuid.New().String()
	cRoot := filepath.Join(root, id)
	assert.NilError(t, os.Mkdir(cRoot, 0o755))
	return NewBaseContainer(id, cRoot)
}

func TestViewSaveDelete(t *testing.T) {
	db, err := NewViewDB()
	assert.NilError(t, err)

	tmpDir := t.TempDir()
	c := newContainer(t, tmpDir)
	assert.NilError(t, c.CheckpointTo(context.Background(), db))
	db.Delete(c.ID)
}

func TestViewAll(t *testing.T) {
	db, err := NewViewDB()
	assert.NilError(t, err)

	tmpDir := t.TempDir()
	one := newContainer(t, tmpDir)
	two := newContainer(t, tmpDir)

	one.State.Pid = 10
	assert.NilError(t, one.CheckpointTo(context.Background(), db))

	two.State.Pid = 20
	assert.NilError(t, two.CheckpointTo(context.Background(), db))

	all, err := db.Snapshot().All()
	assert.NilError(t, err)
	assert.Assert(t, is.Len(all, 2))

	byID := make(map[string]int)
	for _, c := range all {
		byID[c.ID] = c.Pid
	}
	expected := map[string]int{
		one.ID: one.State.Pid,
		two.ID: two.State.Pid,
	}
	assert.DeepEqual(t, expected, byID)
}

func TestViewGet(t *testing.T) {
	db, err := NewViewDB()
	assert.NilError(t, err)

	tmpDir := t.TempDir()
	one := newContainer(t, tmpDir)

	const imgID = "some-image-123"
	one.ImageID = imgID

	assert.NilError(t, one.CheckpointTo(context.Background(), db))
	s, err := db.Snapshot().Get(one.ID)
	assert.NilError(t, err)
	assert.Equal(t, s.ID, one.ID)
}

func TestNames(t *testing.T) {
	db, err := NewViewDB()
	assert.NilError(t, err)

	assert.Check(t, db.ReserveName("name1", "containerid1"))
	assert.Check(t, db.ReserveName("name1", "containerid1")) // idempotent
	assert.Check(t, db.ReserveName("name2", "containerid2"))

	err = db.ReserveName("name2", "containerid3")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsConflict))
	assert.Check(t, is.Error(err, "name is reserved"))

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
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	assert.Check(t, is.Error(err, "name is not reserved"))

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
	db.Delete("containerid1")

	// Reusing one of those names should work
	assert.Check(t, db.ReserveName("name1", "containerid4"))
	view = db.Snapshot()
	assert.Check(t, is.DeepEqual(map[string][]string{"containerid4": {"name1", "name2"}}, view.GetAllNames()))
}

// Test case for GitHub issue 35920
func TestViewWithHealthCheck(t *testing.T) {
	db, err := NewViewDB()
	assert.NilError(t, err)

	tmpDir := t.TempDir()
	one := newContainer(t, tmpDir)

	one.State.Health = &Health{
		Health: container.Health{
			Status: container.Starting,
		},
	}
	assert.NilError(t, one.CheckpointTo(context.Background(), db))
	s, err := db.Snapshot().Get(one.ID)
	assert.NilError(t, err)
	assert.Equal(t, s.Health, container.Starting)
}

func TestTruncIndex(t *testing.T) {
	db, err := NewViewDB()
	assert.NilError(t, err)

	// Get on an empty index
	_, err = db.GetByPrefix("foobar")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))

	// Add an id
	const id = "99b36c2c326ccc11e726eee6ee78a0baf166ef96"
	assert.NilError(t, db.Save(&Container{ID: id}))

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

	// Add an id with a prefix overlapping with the previous one
	id2 := id[:6] + "blabla"
	assert.NilError(t, db.Save(&Container{ID: id2}))

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
	db.Delete(id2)

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

func assertIndexGet(t *testing.T, snapshot *ViewDB, input, expectedResult string, expectError bool) {
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
	for range 100 {
		testSet = append(testSet, stringid.GenerateRandomID())
	}

	for b.Loop() {
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
	for range 100 {
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

	for b.Loop() {
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
	for range 250 {
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

	for b.Loop() {
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
	for range 500 {
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

	for b.Loop() {
		for _, id := range testKeys {
			if res, err := db.GetByPrefix(id); err != nil {
				b.Fatal(res, err)
			}
		}
	}
}

func BenchmarkDBDelete(b *testing.B) {
	var testSet []string
	var testKeys []string
	for i := 0; i < 2500; i++ {
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
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, id := range testKeys {
			db.Delete(id)
		}
	}
}
