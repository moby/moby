//go:build !windows

package file

import (
	"context"

	"github.com/moby/sys/user"
	"github.com/pkg/errors"
	copy "github.com/tonistiigi/fsutil/copy"
)

func mapUserToChowner(user *copy.User, idmap *user.IdentityMapping) (copy.Chowner, error) {
	if user == nil {
		return func(old *copy.User) (*copy.User, error) {
			if old == nil {
				if idmap == nil {
					return nil, nil
				}
				old = &copy.User{} // root
				// non-nil old is already mapped
				if idmap != nil {
					uid, gid, err := idmap.ToHost(old.UID, old.GID)
					if err != nil {
						return nil, errors.WithStack(err)
					}
					return &copy.User{UID: uid, GID: gid}, nil
				}
			}
			return old, nil
		}, nil
	}
	u := *user
	if idmap != nil {
		uid, gid, err := idmap.ToHost(user.UID, user.GID)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		u.UID = uid
		u.GID = gid
	}
	return func(*copy.User) (*copy.User, error) {
		return &u, nil
	}, nil
}

func platformCopy(ctx context.Context, srcRoot string, src string, destRoot string, dest string, opt ...copy.Opt) error {
	return copy.Copy(ctx, srcRoot, src, destRoot, dest, opt...)
}
