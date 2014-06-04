package main

import (
	"log"
	"os"

	"github.com/codegangsta/cli"
	"github.com/dotcloud/docker/pkg/libcontainer"
)

var (
	container *libcontainer.Container
	logPath   = os.Getenv("log")
)

func preload(context *cli.Context) (err error) {
	container, err = loadContainer()
	if err != nil {
		return err
	}

	if logPath != "" {
	}

	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "nsinit"
	app.Version = "0.1"
	app.Author = "libcontainer maintainers"

	app.Before = preload
	app.Commands = []cli.Command{
		execCommand,
		initCommand,
		statsCommand,
		specCommand,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
