package main

import (
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/server"
	"flag"
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
