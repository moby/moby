package main

import (
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/sysinit"
	"log"
	"os"
)

func main() {
	eng, err := engine.New()
	if err != nil {
		log.Fatal(err)
	}

	job := eng.Job(os.Args[0], os.Args[1:]...)

	job.ReplaceEnv(os.Environ())
	job.Stderr.Add(os.Stderr)
	job.Stdout.Add(os.Stdout)
	job.Stdin.Add(os.Stdin)

	// Running in init mode
	os.Exit(int(sysinit.SysInit(job)))
}
