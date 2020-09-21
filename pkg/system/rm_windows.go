package system

import "os"

// EnsureRemoveAll is an alias to os.RemoveAll on Windows
var EnsureRemoveAll = os.RemoveAll
