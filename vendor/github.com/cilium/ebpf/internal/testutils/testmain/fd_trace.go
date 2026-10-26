package testmain

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
)

// foundLeak is atomic since the GC may collect objects in parallel.
var foundLeak atomic.Bool

func onLeakFD(fs *runtime.Frames) {
	foundLeak.Store(true)
	fmt.Fprintln(os.Stderr, "leaked fd created at:")
	fmt.Fprintln(os.Stderr, formatFrames(fs))
}

// fds is a registry of all file descriptors wrapped into sys.fds that were
// created while an fd tracer was active.
var fds *sync.Map // map[int]*runtime.Frames

// TraceFD associates raw with the current execution stack.
//
// skip controls how many entries of the stack the function should skip.
func TraceFD(raw int, skip int) {
	if fds == nil {
		return
	}

	// Attempt to store the caller's stack for the given fd value.
	// Panic if fds contains an existing stack for the fd.
	old, exist := fds.LoadOrStore(raw, callersFrames(skip))
	if exist {
		f := old.(*runtime.Frames)
		panic(fmt.Sprintf("found existing stack for fd %d:\n%s", raw, formatFrames(f)))
	}
}

// ForgetFD removes any existing association for raw.
func ForgetFD(raw int) {
	if fds != nil {
		fds.Delete(raw)
	}
}

// LeakFD indicates that raw was leaked.
//
// Calling the function with a value that was not passed to [TraceFD] before
// is undefined.
func LeakFD(raw int) {
	if fds == nil {
		return
	}

	// Invoke the fd leak callback. Calls LoadAndDelete to guarantee the callback
	// is invoked at most once for one sys.FD allocation, runtime.Frames can only
	// be unwound once.
	f, ok := fds.LoadAndDelete(raw)
	if ok {
		onLeakFD(f.(*runtime.Frames))
	}
}

// flushFrames removes all elements from fds and returns them as a slice. This
// deals with the fact that a runtime.Frames can only be unwound once using
// Next().
func flushFrames() []*runtime.Frames {
	var frames []*runtime.Frames
	fds.Range(func(key, value any) bool {
		frames = append(frames, value.(*runtime.Frames))
		fds.Delete(key)
		return true
	})
	return frames
}

func callersFrames(skip int) *runtime.Frames {
	c := make([]uintptr, 32)

	// Skip runtime.Callers and this function.
	i := runtime.Callers(skip+2, c)
	if i == 0 {
		return nil
	}

	return runtime.CallersFrames(c)
}

// formatFrames formats a runtime.Frames as a human-readable string.
func formatFrames(fs *runtime.Frames) string {
	var b bytes.Buffer
	for {
		f, more := fs.Next()
		b.WriteString(fmt.Sprintf("\t%s+%#x\n\t\t%s:%d\n", f.Function, f.PC-f.Entry, f.File, f.Line))
		if !more {
			break
		}
	}
	return b.String()
}
