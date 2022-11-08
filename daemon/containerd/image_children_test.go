package containerd

import (
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestIsRootfsChildOf(t *testing.T) {
	// Each unique letter is one distinct DiffID
	ab := toRootfs("AB")
	abc := toRootfs("ABC")
	abd := toRootfs("ABD")
	xyz := toRootfs("XYZ")
	xyzab := toRootfs("XYZAB")

	for _, tc := range []struct {
		name   string
		parent ocispec.RootFS
		child  ocispec.RootFS
		out    bool
	}{
		{parent: ab, child: abc, out: true, name: "one additional layer"},
		{parent: xyz, child: xyzab, out: true, name: "two additional layers"},
		{parent: xyz, child: xyz, out: false, name: "parent is not a child of itself"},
		{parent: abc, child: abd, out: false, name: "sibling"},
		{parent: abc, child: xyz, out: false, name: "completely different rootfs, but same length"},
		{parent: abc, child: ab, out: false, name: "child can't be shorter than parent"},
		{parent: ab, child: xyzab, out: false, name: "parent layers appended"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			out := isRootfsChildOf(tc.child, tc.parent)

			assert.Check(t, is.Equal(out, tc.out))
		})
	}
}

func toRootfs(values string) ocispec.RootFS {
	dgsts := []digest.Digest{}

	for _, v := range values {
		vd := digest.FromString(string(v))
		dgsts = append(dgsts, vd)
	}

	return ocispec.RootFS{
		Type:    "layers",
		DiffIDs: dgsts,
	}
}
