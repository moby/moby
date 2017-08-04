package sysx

import (
	"errors"
)

// Initial stub version for FreeBSD. FreeBSD has a different
// syscall API from Darwin and Linux for extended attributes;
// it is also not widely used. It is not exposed at all by the
// Go syscall package, so we need to implement directly eventually.

var unsupported = errors.New("extended attributes unsupported on FreeBSD")
