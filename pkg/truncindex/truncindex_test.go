package truncindex

import (
	"math/rand"
	"testing"

	"github.com/docker/docker/pkg/stringid"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

// Test the behavior of TruncIndex, an index for querying IDs from a non-conflicting prefix.
func (s *DockerSuite) TestTruncIndex(c *check.C) {
	ids := []string{}
	index := NewTruncIndex(ids)
	// Get on an empty index
	if _, err := index.Get("foobar"); err == nil {
		c.Fatal("Get on an empty index should return an error")
	}

	// Spaces should be illegal in an id
	if err := index.Add("I have a space"); err == nil {
		c.Fatalf("Adding an id with ' ' should return an error")
	}

	id := "99b36c2c326ccc11e726eee6ee78a0baf166ef96"
	// Add an id
	if err := index.Add(id); err != nil {
		c.Fatal(err)
	}

	// Add an empty id (should fail)
	if err := index.Add(""); err == nil {
		c.Fatalf("Adding an empty id should return an error")
	}

	// Get a non-existing id
	assertIndexGet(c, index, "abracadabra", "", true)
	// Get an empty id
	assertIndexGet(c, index, "", "", true)
	// Get the exact id
	assertIndexGet(c, index, id, id, false)
	// The first letter should match
	assertIndexGet(c, index, id[:1], id, false)
	// The first half should match
	assertIndexGet(c, index, id[:len(id)/2], id, false)
	// The second half should NOT match
	assertIndexGet(c, index, id[len(id)/2:], "", true)

	id2 := id[:6] + "blabla"
	// Add an id
	if err := index.Add(id2); err != nil {
		c.Fatal(err)
	}
	// Both exact IDs should work
	assertIndexGet(c, index, id, id, false)
	assertIndexGet(c, index, id2, id2, false)

	// 6 characters or less should conflict
	assertIndexGet(c, index, id[:6], "", true)
	assertIndexGet(c, index, id[:4], "", true)
	assertIndexGet(c, index, id[:1], "", true)

	// An ambiguous id prefix should return an error
	if _, err := index.Get(id[:4]); err == nil {
		c.Fatal("An ambiguous id prefix should return an error")
	}

	// 7 characters should NOT conflict
	assertIndexGet(c, index, id[:7], id, false)
	assertIndexGet(c, index, id2[:7], id2, false)

	// Deleting a non-existing id should return an error
	if err := index.Delete("non-existing"); err == nil {
		c.Fatalf("Deleting a non-existing id should return an error")
	}

	// Deleting an empty id should return an error
	if err := index.Delete(""); err == nil {
		c.Fatal("Deleting an empty id should return an error")
	}

	// Deleting id2 should remove conflicts
	if err := index.Delete(id2); err != nil {
		c.Fatal(err)
	}
	// id2 should no longer work
	assertIndexGet(c, index, id2, "", true)
	assertIndexGet(c, index, id2[:7], "", true)
	assertIndexGet(c, index, id2[:11], "", true)

	// conflicts between id and id2 should be gone
	assertIndexGet(c, index, id[:6], id, false)
	assertIndexGet(c, index, id[:4], id, false)
	assertIndexGet(c, index, id[:1], id, false)

	// non-conflicting substrings should still not conflict
	assertIndexGet(c, index, id[:7], id, false)
	assertIndexGet(c, index, id[:15], id, false)
	assertIndexGet(c, index, id, id, false)

	assertIndexIterate(c)
}

func assertIndexIterate(c *check.C) {
	ids := []string{
		"19b36c2c326ccc11e726eee6ee78a0baf166ef96",
		"28b36c2c326ccc11e726eee6ee78a0baf166ef96",
		"37b36c2c326ccc11e726eee6ee78a0baf166ef96",
		"46b36c2c326ccc11e726eee6ee78a0baf166ef96",
	}

	index := NewTruncIndex(ids)

	index.Iterate(func(targetId string) {
		for _, id := range ids {
			if targetId == id {
				return
			}
		}

		c.Fatalf("An unknown ID '%s'", targetId)
	})
}

func assertIndexGet(c *check.C, index *TruncIndex, input, expectedResult string, expectError bool) {
	if result, err := index.Get(input); err != nil && !expectError {
		c.Fatalf("Unexpected error getting '%s': %s", input, err)
	} else if err == nil && expectError {
		c.Fatalf("Getting '%s' should return an error, not '%s'", input, result)
	} else if result != expectedResult {
		c.Fatalf("Getting '%s' returned '%s' instead of '%s'", input, result, expectedResult)
	}
}

func (s *DockerSuite) BenchmarkTruncIndexAdd100(c *check.C) {
	var testSet []string
	for i := 0; i < 100; i++ {
		testSet = append(testSet, stringid.GenerateNonCryptoID())
	}
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		index := NewTruncIndex([]string{})
		for _, id := range testSet {
			if err := index.Add(id); err != nil {
				c.Fatal(err)
			}
		}
	}
}

func (s *DockerSuite) BenchmarkTruncIndexAdd250(c *check.C) {
	var testSet []string
	for i := 0; i < 250; i++ {
		testSet = append(testSet, stringid.GenerateNonCryptoID())
	}
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		index := NewTruncIndex([]string{})
		for _, id := range testSet {
			if err := index.Add(id); err != nil {
				c.Fatal(err)
			}
		}
	}
}

func (s *DockerSuite) BenchmarkTruncIndexAdd500(c *check.C) {
	var testSet []string
	for i := 0; i < 500; i++ {
		testSet = append(testSet, stringid.GenerateNonCryptoID())
	}
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		index := NewTruncIndex([]string{})
		for _, id := range testSet {
			if err := index.Add(id); err != nil {
				c.Fatal(err)
			}
		}
	}
}

func (s *DockerSuite) BenchmarkTruncIndexGet100(c *check.C) {
	var testSet []string
	var testKeys []string
	for i := 0; i < 100; i++ {
		testSet = append(testSet, stringid.GenerateNonCryptoID())
	}
	index := NewTruncIndex([]string{})
	for _, id := range testSet {
		if err := index.Add(id); err != nil {
			c.Fatal(err)
		}
		l := rand.Intn(12) + 12
		testKeys = append(testKeys, id[:l])
	}
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		for _, id := range testKeys {
			if res, err := index.Get(id); err != nil {
				c.Fatal(res, err)
			}
		}
	}
}

func (s *DockerSuite) BenchmarkTruncIndexGet250(c *check.C) {
	var testSet []string
	var testKeys []string
	for i := 0; i < 250; i++ {
		testSet = append(testSet, stringid.GenerateNonCryptoID())
	}
	index := NewTruncIndex([]string{})
	for _, id := range testSet {
		if err := index.Add(id); err != nil {
			c.Fatal(err)
		}
		l := rand.Intn(12) + 12
		testKeys = append(testKeys, id[:l])
	}
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		for _, id := range testKeys {
			if res, err := index.Get(id); err != nil {
				c.Fatal(res, err)
			}
		}
	}
}

func (s *DockerSuite) BenchmarkTruncIndexGet500(c *check.C) {
	var testSet []string
	var testKeys []string
	for i := 0; i < 500; i++ {
		testSet = append(testSet, stringid.GenerateNonCryptoID())
	}
	index := NewTruncIndex([]string{})
	for _, id := range testSet {
		if err := index.Add(id); err != nil {
			c.Fatal(err)
		}
		l := rand.Intn(12) + 12
		testKeys = append(testKeys, id[:l])
	}
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		for _, id := range testKeys {
			if res, err := index.Get(id); err != nil {
				c.Fatal(res, err)
			}
		}
	}
}

func (s *DockerSuite) BenchmarkTruncIndexDelete100(c *check.C) {
	var testSet []string
	for i := 0; i < 100; i++ {
		testSet = append(testSet, stringid.GenerateNonCryptoID())
	}
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		c.StopTimer()
		index := NewTruncIndex([]string{})
		for _, id := range testSet {
			if err := index.Add(id); err != nil {
				c.Fatal(err)
			}
		}
		c.StartTimer()
		for _, id := range testSet {
			if err := index.Delete(id); err != nil {
				c.Fatal(err)
			}
		}
	}
}

func (s *DockerSuite) BenchmarkTruncIndexDelete250(c *check.C) {
	var testSet []string
	for i := 0; i < 250; i++ {
		testSet = append(testSet, stringid.GenerateNonCryptoID())
	}
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		c.StopTimer()
		index := NewTruncIndex([]string{})
		for _, id := range testSet {
			if err := index.Add(id); err != nil {
				c.Fatal(err)
			}
		}
		c.StartTimer()
		for _, id := range testSet {
			if err := index.Delete(id); err != nil {
				c.Fatal(err)
			}
		}
	}
}

func (s *DockerSuite) BenchmarkTruncIndexDelete500(c *check.C) {
	var testSet []string
	for i := 0; i < 500; i++ {
		testSet = append(testSet, stringid.GenerateNonCryptoID())
	}
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		c.StopTimer()
		index := NewTruncIndex([]string{})
		for _, id := range testSet {
			if err := index.Add(id); err != nil {
				c.Fatal(err)
			}
		}
		c.StartTimer()
		for _, id := range testSet {
			if err := index.Delete(id); err != nil {
				c.Fatal(err)
			}
		}
	}
}

func (s *DockerSuite) BenchmarkTruncIndexNew100(c *check.C) {
	var testSet []string
	for i := 0; i < 100; i++ {
		testSet = append(testSet, stringid.GenerateNonCryptoID())
	}
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		NewTruncIndex(testSet)
	}
}

func (s *DockerSuite) BenchmarkTruncIndexNew250(c *check.C) {
	var testSet []string
	for i := 0; i < 250; i++ {
		testSet = append(testSet, stringid.GenerateNonCryptoID())
	}
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		NewTruncIndex(testSet)
	}
}

func (s *DockerSuite) BenchmarkTruncIndexNew500(c *check.C) {
	var testSet []string
	for i := 0; i < 500; i++ {
		testSet = append(testSet, stringid.GenerateNonCryptoID())
	}
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		NewTruncIndex(testSet)
	}
}

func (s *DockerSuite) BenchmarkTruncIndexAddGet100(c *check.C) {
	var testSet []string
	var testKeys []string
	for i := 0; i < 500; i++ {
		id := stringid.GenerateNonCryptoID()
		testSet = append(testSet, id)
		l := rand.Intn(12) + 12
		testKeys = append(testKeys, id[:l])
	}
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		index := NewTruncIndex([]string{})
		for _, id := range testSet {
			if err := index.Add(id); err != nil {
				c.Fatal(err)
			}
		}
		for _, id := range testKeys {
			if res, err := index.Get(id); err != nil {
				c.Fatal(res, err)
			}
		}
	}
}

func (s *DockerSuite) BenchmarkTruncIndexAddGet250(c *check.C) {
	var testSet []string
	var testKeys []string
	for i := 0; i < 500; i++ {
		id := stringid.GenerateNonCryptoID()
		testSet = append(testSet, id)
		l := rand.Intn(12) + 12
		testKeys = append(testKeys, id[:l])
	}
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		index := NewTruncIndex([]string{})
		for _, id := range testSet {
			if err := index.Add(id); err != nil {
				c.Fatal(err)
			}
		}
		for _, id := range testKeys {
			if res, err := index.Get(id); err != nil {
				c.Fatal(res, err)
			}
		}
	}
}

func (s *DockerSuite) BenchmarkTruncIndexAddGet500(c *check.C) {
	var testSet []string
	var testKeys []string
	for i := 0; i < 500; i++ {
		id := stringid.GenerateNonCryptoID()
		testSet = append(testSet, id)
		l := rand.Intn(12) + 12
		testKeys = append(testKeys, id[:l])
	}
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		index := NewTruncIndex([]string{})
		for _, id := range testSet {
			if err := index.Add(id); err != nil {
				c.Fatal(err)
			}
		}
		for _, id := range testKeys {
			if res, err := index.Get(id); err != nil {
				c.Fatal(res, err)
			}
		}
	}
}
