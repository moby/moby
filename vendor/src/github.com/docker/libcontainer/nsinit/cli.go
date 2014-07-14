package nsinit

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

func NsInit() {
	app := cli.NewApp()
	app.Name = "nsinit"
	app.Version = "0.1"
	app.Author = "libcontainer maintainers"

	app.Before = preload
	app.Commands = []cli.Command{
		execCommand,
		initCommand,
		statsCommand,
		configCommand,
		nsenterCommand,
		pauseCommand,
		unpauseCommand,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
