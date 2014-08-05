// +build !daemon

package main

import (
	"log"
)

const CanDaemon = false

func mainSysinit() {
	log.Fatal("This is a client-only binary - running it as 'dockerinit' is not supported.")
}

func mainDaemon() {
	log.Fatal("This is a client-only binary - running the Docker daemon is not supported.")
}
