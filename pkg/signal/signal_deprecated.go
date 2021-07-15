package signal

import "github.com/docker/docker/pkg/stack"

// DumpStacks appends the runtime stack into file in dir and returns full path
// to that file.
// Deprecated: use github.com/docker/docker/pkg/stack.Dump instead.
var DumpStacks = stack.DumpToFile
