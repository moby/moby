package main

import (
	"fmt"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/engine/spawn"
	"log"
	"os"
	"os/exec"
	"strings"
)

func main() {
	fmt.Printf("[%d] MAIN\n", os.Getpid())
	spawn.Init(&Worker{})
	fmt.Printf("[%d parent] spawning\n", os.Getpid())
	eng, err := spawn.Spawn()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("[parent] spawned\n")
	job := eng.Job(os.Args[1], os.Args[2:]...)
	job.Stdout.Add(os.Stdout)
	job.Stderr.Add(os.Stderr)
	job.Run()
	// FIXME: use the job's status code
	os.Exit(0)
}

type Worker struct {
}

func (w *Worker) Install(eng *engine.Engine) error {
	eng.Register("exec", w.Exec)
	eng.Register("cd", w.Cd)
	eng.Register("echo", w.Echo)
	return nil
}

func (w *Worker) Exec(job *engine.Job) engine.Status {
	fmt.Printf("--> %v\n", job.Args)
	cmd := exec.Command(job.Args[0], job.Args[1:]...)
	cmd.Stdout = job.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return job.Errorf("%v\n", err)
	}
	return engine.StatusOK
}

func (w *Worker) Cd(job *engine.Job) engine.Status {
	if err := os.Chdir(job.Args[0]); err != nil {
		return job.Errorf("%v\n", err)
	}
	return engine.StatusOK
}

func (w *Worker) Echo(job *engine.Job) engine.Status {
	fmt.Fprintf(job.Stdout, "%s\n", strings.Join(job.Args, " "))
	return engine.StatusOK
}
