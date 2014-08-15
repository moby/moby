package main

import (
	"log"
	"os"
	"runtime"
	"strconv"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer/namespaces"
	"github.com/docker/libcontainer/syncpipe"
)

var (
	dataPath  = os.Getenv("data_path")
	console   = os.Getenv("console")
	rawPipeFd = os.Getenv("pipe")

	initCommand = cli.Command{
		Name:   "init",
		Usage:  "runs the init process inside the namespace",
		Action: initAction,
	}
)

func initAction(context *cli.Context) {
	runtime.LockOSThread()

	container, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	rootfs, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	pipeFd, err := strconv.Atoi(rawPipeFd)
	if err != nil {
		log.Fatal(err)
	}

	syncPipe, err := syncpipe.NewSyncPipeFromFd(0, uintptr(pipeFd))
	if err != nil {
		log.Fatalf("unable to create sync pipe: %s", err)
	}

	if err := namespaces.Init(container, rootfs, console, syncPipe, []string(context.Args())); err != nil {
		log.Fatalf("unable to initialize for container: %s", err)
	}
}
