package main

import (
	"log"
	"os"

	"github.com/codegangsta/cli"
)

var logPath = os.Getenv("log")

func preload(context *cli.Context) error {
	if logPath != "" {
		if err := openLog(logPath); err != nil {
			return err
		}
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
		nsenterCommand,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
