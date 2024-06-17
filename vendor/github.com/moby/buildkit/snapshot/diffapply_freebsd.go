package snapshot

import (
	"context"

	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/snapshots"
	"github.com/pkg/errors"
)

func (sn *mergeSnapshotter) diffApply(_ context.Context, _ Mountable, _ ...Diff) (_ snapshots.Usage, rerr error) {
	return snapshots.Usage{}, errors.New("diffApply not yet supported on FreeBSD")
}

func needsUserXAttr(_ context.Context, _ Snapshotter, _ leases.Manager) (bool, error) {
	return false, errors.New("needs userxattr not supported on FreeBSD")
}
