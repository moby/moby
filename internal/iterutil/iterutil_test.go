package iterutil

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestSameValues(t *testing.T) {
	a := []int{1, 2, 3, 4, 3}
	b := []int{3, 4, 3, 2, 1}
	c := []int{1, 2, 3, 4}

	assert.Check(t, SameValues(slices.Values(a), slices.Values(a)))
	assert.Check(t, SameValues(slices.Values(c), slices.Values(c)))
	assert.Check(t, SameValues(slices.Values(a), slices.Values(b)))
	assert.Check(t, !SameValues(slices.Values(a), slices.Values(c)))
}

func TestDeref(t *testing.T) {
	a := make([]*int, 3)
	for i := range a {
		a[i] = &i
	}
	b := slices.Collect(Deref(slices.Values(a)))
	assert.DeepEqual(t, b, []int{0, 1, 2})
}

func TestChain(t *testing.T) {
	a := []int{1, 2, 3}
	b := []int{4, 5}
	c := []int{6}

	ab := Chain(slices.Values(a), slices.Values(b))
	abc := Chain(ab, slices.Values(c))

	assert.DeepEqual(t, slices.Collect(ab), []int{1, 2, 3, 4, 5})
	assert.DeepEqual(t, slices.Collect(abc), []int{1, 2, 3, 4, 5, 6})
}

func TestChain2(t *testing.T) {
	a := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
	}
	b := map[string]int{
		"d": 4,
		"e": 5,
	}
	c := map[string]int{
		"f": 6,
	}

	ab := Chain2(maps.All(a), maps.All(b))
	abc := Chain2(ab, maps.All(c))

	expab := maps.Clone(a)
	maps.Insert(expab, maps.All(b))

	expabc := maps.Clone(expab)
	maps.Insert(expabc, maps.All(c))

	assert.DeepEqual(t, maps.Collect(ab), expab)
	assert.DeepEqual(t, maps.Collect(abc), expabc)
}

func TestMap(t *testing.T) {
	a := []int{1, 2, 3}
	b := slices.Collect(Map(slices.Values(a), strconv.Itoa))
	assert.DeepEqual(t, b, []string{"1", "2", "3"})
}

func TestMap2(t *testing.T) {
	a := map[string]int{"a": 1, "b": 2, "c": 3}
	b := maps.Collect(Map2(maps.All(a), func(k string, v int) (string, string) {
		return strings.ToUpper(k), strconv.Itoa(v)
	}))
	assert.DeepEqual(t, b, map[string]string{"A": "1", "B": "2", "C": "3"})
}
