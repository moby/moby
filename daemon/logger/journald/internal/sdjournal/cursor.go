//go:build linux && cgo && !static_build && journald
// +build linux,cgo,!static_build,journald

package sdjournal // import "github.com/docker/docker/daemon/logger/journald/internal/sdjournal"

// #include <stdlib.h>
import "C"
import (
	"runtime"
	"unsafe"
)

// Cursor is a reference to a journal cursor. A Cursor must not be copied.
type Cursor struct {
	c      *C.char
	noCopy noCopy //nolint:structcheck,unused // Exists only to mark values uncopyable for `go vet`.
}

func wrapCursor(cur *C.char) *Cursor {
	c := &Cursor{c: cur}
	runtime.SetFinalizer(c, (*Cursor).Free)
	return c
}

func (c *Cursor) String() string {
	if c.c == nil {
		return "<nil>"
	}
	return C.GoString(c.c)
}

// Free invalidates the cursor and frees any associated resources on the C heap.
func (c *Cursor) Free() {
	if c == nil {
		return
	}
	C.free(unsafe.Pointer(c.c))
	runtime.SetFinalizer(c, nil)
	c.c = nil
}

type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}
