package main

import (
	"log"
	"flag"
	"github.com/dotcloud/docker/server"
	"github.com/dotcloud/docker"
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
