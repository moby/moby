package main

import (
	"flag"
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/server"
	"log"
)

func main() {
	if docker.SelfPath() == "/sbin/init" {
		// Running in init mode
		docker.SysInit()
		return
	}
	flag.Parse()
	d, err := server.New()
	if err != nil {
		log.Fatal(err)
	}
	if err := d.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
