// +build !darwin

package main

import (
	"log"
)

func run(cpus, mem, diskSz int, userData string, args []string) {
	log.Fatalf("'run' is not support yet on your OS")
}
