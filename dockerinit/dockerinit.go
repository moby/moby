package main

import (
	"github.com/dotcloud/docker/sysinit"
)

func main() {
	// Running in init mode
	sysinit.SysInit()
	return
}
