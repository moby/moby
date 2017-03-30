// +build !darwin

package main

import (
	log "github.com/Sirupsen/logrus"
)

func run(args []string) {
	log.Fatalf("'run' is not supported yet on your OS")
}
