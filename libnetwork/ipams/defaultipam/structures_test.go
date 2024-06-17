package defaultipam

import (
	"net/netip"
	"testing"

	"github.com/docker/docker/libnetwork/internal/netiputil"
	"gotest.tools/v3/assert"
)

func TestMergeIter(t *testing.T) {
	allocated := []netip.Prefix{
		netip.MustParsePrefix("172.16.0.0/24"),
		netip.MustParsePrefix("172.17.0.0/24"),
		netip.MustParsePrefix("172.18.0.0/24"),
	}
	reserved := []netip.Prefix{
		netip.MustParsePrefix("172.16.0.0/24"),
	}
	it := newMergeIter(allocated, reserved, netiputil.PrefixCompare)

	for _, exp := range []netip.Prefix{
		allocated[0],
		reserved[0],
		allocated[1],
		allocated[2],
		{},
	} {
		assert.Equal(t, it.Get(), exp)
		it.Inc()
	}
}
