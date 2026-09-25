package file

import (
	"context"
	"path/filepath"

	"github.com/moby/buildkit/util/windows"
	"github.com/moby/sys/user"
	copy "github.com/tonistiigi/fsutil/copy"
)

func mapUserToChowner(user *copy.User, _ *user.IdentityMapping) (copy.Chowner, error) {
	if user == nil || user.SID == "" {
		return func(old *copy.User) (*copy.User, error) {
			if old == nil || old.SID == "" {
				old = &copy.User{
					SID: windows.ContainerAdministratorSidString,
				}
			}
			return old, nil
		}, nil
	}
	return func(*copy.User) (*copy.User, error) {
		return user, nil
	}, nil
}

// platformCopy wraps copy.Copy to exclude Windows protected system folders.
// On Windows, container snapshots mounted to the host filesystem include protected folders
// ("System Volume Information" and "WcSandboxState") at the mount root, which cause "Access is denied"
// errors. With the fsutil fix, these are excluded before os.Lstat() is called.
func platformCopy(ctx context.Context, srcRoot string, src string, destRoot string, dest string, opt ...copy.Opt) error {
	// Only exclude protected folders when copying from the mount root.
	if filepath.Clean(src) == string(filepath.Separator) {
		opt = append(opt,
			copy.WithExcludePattern("System Volume Information"),
			copy.WithExcludePattern("WcSandboxState"),
		)
	}
	return copy.Copy(ctx, srcRoot, src, destRoot, dest, opt...)
}
