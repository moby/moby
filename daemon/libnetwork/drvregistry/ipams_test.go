package drvregistry

import (
	"runtime"
	"sort"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/ipamapi"
	"github.com/moby/moby/v2/daemon/libnetwork/ipams"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func getNewIPAMs(t *testing.T) *IPAMs {
	r := &IPAMs{}

	assert.Assert(t, ipams.Register(r, nil, nil, nil, nil))

	return r
}

func TestIPAMs(t *testing.T) {
	t.Run("IPAM", func(t *testing.T) {
		reg := getNewIPAMs(t)

		i, caps := reg.IPAM("default")
		assert.Check(t, i != nil)
		assert.Check(t, caps != nil)
	})

	t.Run("WalkIPAMs", func(t *testing.T) {
		reg := getNewIPAMs(t)

		ipams := make([]string, 0, 2)
		reg.WalkIPAMs(func(name string, driver ipamapi.Ipam, capability *ipamapi.Capability) bool {
			ipams = append(ipams, name)
			return false
		})

		sort.Strings(ipams)
		expected := []string{"default", "null"}
		if runtime.GOOS == "windows" {
			expected = append(expected, "windows")
		}
		assert.Check(t, is.DeepEqual(ipams, expected))
	})
}
