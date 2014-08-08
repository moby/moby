package nsinit

import (
	"log"
	"os"

	"github.com/codegangsta/cli"
)

var (
	logPath = os.Getenv("log")
	argvs   = make(map[string]func())
)

func init() {
	argvs["nsenter"] = nsenter
}

func preload(context *cli.Context) error {
	if logPath != "" {
		if err := openLog(logPath); err != nil {
			return err
		}
	}

	return nil
}

func NsInit() {
	// we need to check our argv 0 for any registred functions to run instead of the
	// normal cli code path

	action, exists := argvs[os.Args[0]]
	if exists {
		action()

		return
	}

	app := cli.NewApp()

	app.Name = "nsinit"
	app.Version = "0.1"
	app.Author = "libcontainer maintainers"
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "nspid"},
		cli.StringFlag{Name: "console"},
	}

	app.Before = preload

	app.Commands = []cli.Command{
		execCommand,
		initCommand,
		statsCommand,
		configCommand,
		pauseCommand,
		unpauseCommand,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
