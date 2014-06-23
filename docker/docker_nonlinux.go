// +build !linux

package main

import (
	"log"
)

func trySysInit() {
	log.Fatalf("trySysInit not implemented on non-linux")
}

func daemonCommand() {
	log.Fatalf("The Docker daemon is only supported on linux")
}
