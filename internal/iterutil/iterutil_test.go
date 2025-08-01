package iterutil

import (
	"slices"
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
