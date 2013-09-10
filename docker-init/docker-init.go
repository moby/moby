package main

import (
	"github.com/dotcloud/docker"
)

var (
	GITCOMMIT string
	VERSION   string
)

func main() {
	// Running in init mode
	docker.SysInit()
	return
}
