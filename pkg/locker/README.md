Locker
=====

locker provides a mechanism for creating finer-grained locking to help
free up more global locks to handle other tasks.

The implementation looks close to a sync.Mutex, however the user must provide a
reference to use to refer to the underlying lock when locking and unlocking,
and unlock may generate an error.

If a lock with a given name does not exist when `Lock` is called, one is
created.
Lock references are automatically cleaned up on `Unlock` if nothing else is
waiting for the lock.


## Usage

```go
package important

import (
	"sync"
	"time"

	"github.com/docker/docker/pkg/locker"
)

type important struct {
	locks *locker.Locker
	data  map[string]interface{}
	mu    sync.Mutex
}

func (i *important) Get(name string) interface{} {
	i.locks.Lock(name)
	defer i.locks.Unlock(name)
	return data[name]
}

func (i *important) Create(name string, data interface{}) {
	i.locks.Lock(name)
	defer i.locks.Unlock(name)

	i.createImportant(data)

	s.mu.Lock()
	i.data[name] = data
	s.mu.Unlock()
}

func (i *important) createImportant(data interface{}) {
	time.Sleep(10 * time.Second)
}
```

For functions dealing with a given name, always lock at the beginning of the
function (or before doing anything with the underlying state), this ensures any
other function that is dealing with the same name will block.

When needing to modify the underlying data, use the global lock to ensure nothing
else is modfying it at the same time.
Since name lock is already in place, no reads will occur while the modification
is being performed.

### CancelWithError

CancelWithError manually set the error for the lock with the given name.
Other goroutines who are waiting for the lock will get an error, instead of getting the lock.

```go
# In goroutine1
locker.Lock("test")
defer locker.Lock("test")
locker.CancelWithError("test", errors.New("some error"))

# In goroutine2, assuming that it runs after goroutine1 getting the lock
if err := locker.Lock("test"); err != nil {
  # goroutine1 called the CancelWithError when it was holding the lock,
  # this Lock function will return that error, indicating that this lock action has been canceled.
  # ... handle the error ...
} else {
  defer locker.Unlock("test")
}
```
