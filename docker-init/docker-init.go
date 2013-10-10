package main

import (
	"github.com/dotcloud/docker/sysinit"
)

var (
	GITCOMMIT string
	VERSION   string
)

func main() {
	// Running in init mode
	sysinit.SysInit()
	return
}
