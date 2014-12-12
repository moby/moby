package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
)

func initLogging(lvl log.Level) {
	log.SetOutput(os.Stderr)
	log.SetLevel(lvl)
}
