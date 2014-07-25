package main

import (
	"github.com/docker/docker/sysinit"
)

func main() {
	// Running in init mode
	sysinit.SysInit()
	return
}
