//go:build (!((amd64 || arm64 || riscv64) && linux) && !((amd64 || arm64) && (darwin || freebsd)) && !((amd64 || arm64) && windows)) || js

package sys

import "io/fs"

// sysParseable is only used here as we define "supported" as being able to
// parse `info.Sys()`. The above `go:build` constraints exclude 32-bit until
// that's requested.
const sysParseable = false

func statFromFileInfo(info fs.FileInfo) Stat_t {
	return defaultStatFromFileInfo(info)
}
