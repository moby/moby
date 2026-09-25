package networkdb

import (
	"testing"

	"github.com/hashicorp/memberlist"
	"gotest.tools/v3/assert"
)

func TestNodeCanReceiveNetworkEvents(t *testing.T) {
	for _, tc := range []struct {
		name string
		meta []byte
		want bool
	}{
		{"no metadata at all", nil, false},
		{"empty metadata, as every daemon before this sent", []byte{}, false},
		{"advertises ltime-aware invalidation", []byte{nodeMetaLTimeInvalidation}, true},
		{"advertises something later", []byte{nodeMetaLTimeInvalidation + 1}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := &node{Node: memberlist.Node{Meta: tc.meta}}
			assert.Equal(t, n.canReceiveNetworkEvents(), tc.want)
		})
	}
}

// What this daemon advertises has to satisfy its own predicate, or a cluster of
// current daemons would never send each other attachments at all.
func TestNodeMetaAdvertisesOwnCapability(t *testing.T) {
	d := &delegate{}
	n := &node{Node: memberlist.Node{Meta: d.NodeMeta(memberlist.MetaMaxSize)}}
	assert.Check(t, n.canReceiveNetworkEvents())
	assert.Check(t, len(n.Meta) <= memberlist.MetaMaxSize)
}
