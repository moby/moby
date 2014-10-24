package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
)

func initLogging(debug bool) {
	log.SetOutput(os.Stderr)
	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
}
