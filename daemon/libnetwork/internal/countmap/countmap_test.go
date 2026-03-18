package countmap_test

import (
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/internal/countmap"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestMap(t *testing.T) {
	m := countmap.Map[string]{}
	m["foo"] = 7
	m["bar"] = 2
	m["zeroed"] = -2

	m.Add("bar", -3)
	m.Add("foo", -8)
	m.Add("baz", 1)
	m.Add("zeroed", 2)
	assert.Check(t, is.DeepEqual(m, countmap.Map[string]{"foo": -1, "bar": -1, "baz": 1}))

	m.Add("foo", 1)
	m.Add("bar", 1)
	m.Add("baz", -1)
	assert.Check(t, is.DeepEqual(m, countmap.Map[string]{}))
}
