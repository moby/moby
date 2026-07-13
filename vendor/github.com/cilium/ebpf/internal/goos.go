package internal

import "runtime"

const (
	OnLinux = runtime.GOOS == "linux"
)
