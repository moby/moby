package main

import (
	log "github.com/Sirupsen/logrus"
	"io"
)

func setLogLevel(lvl log.Level) {
	log.SetLevel(lvl)
}

func initLogging(stderr io.Writer) {
	log.SetOutput(stderr)
}
