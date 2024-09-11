//go:build !windows
// +build !windows

package file

import (
	"github.com/docker/docker/pkg/idtools"
	"github.com/pkg/errors"
	copy "github.com/tonistiigi/fsutil/copy"
)

func mapUserToChowner(user *copy.User, idmap *idtools.IdentityMapping) (copy.Chowner, error) {
	if user == nil {
		return func(old *copy.User) (*copy.User, error) {
			if old == nil {
				if idmap == nil {
					return nil, nil
				}
				old = &copy.User{} // root
				// non-nil old is already mapped
				if idmap != nil {
					identity, err := idmap.ToHost(idtools.Identity{
						UID: old.UID,
						GID: old.GID,
					})
					if err != nil {
						return nil, errors.WithStack(err)
					}
					return &copy.User{UID: identity.UID, GID: identity.GID}, nil
				}
			}
			return old, nil
		}, nil
	}
	u := *user
	if idmap != nil {
		identity, err := idmap.ToHost(idtools.Identity{
			UID: user.UID,
			GID: user.GID,
		})
		if err != nil {
			return nil, errors.WithStack(err)
		}
		u.UID = identity.UID
		u.GID = identity.GID
	}
	return func(*copy.User) (*copy.User, error) {
		return &u, nil
	}, nil
}
