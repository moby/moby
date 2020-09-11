/*
Package locker provides a mechanism for creating finer-grained locking to help
free up more global locks to handle other tasks.

The implementation looks close to a sync.Mutex, however the user must provide a
reference to use to refer to the underlying lock when locking and unlocking,
and unlock may generate an error.

If a lock with a given name does not exist when `Lock` is called, one is
created.
Lock references are automatically cleaned up on `Unlock` if nothing else is
waiting for the lock.
*/
package locker // import "github.com/docker/docker/pkg/locker"

import (
	"github.com/moby/locker"
)

// ErrNoSuchLock is returned when the requested lock does not exist
// Deprecated: use github.com/moby/locker.ErrNoSuchLock
var ErrNoSuchLock = locker.ErrNoSuchLock

// Locker provides a locking mechanism based on the passed in reference name
// Deprecated: use github.com/moby/locker.Locker
type Locker = locker.Locker

// New creates a new Locker
// Deprecated: use github.com/moby/locker.New
var New = locker.New
