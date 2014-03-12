// Simple application to run multiple processes like supervisord
package main

import (
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/pkg/signal"
	"github.com/dotcloud/docker/pkg/supervisor"
	"log"
	"os"
	"syscall"
	"time"
)

type process struct {
	Args      []string `json:"args"`
	AttachStd bool     `json:"attach_std"`
}

func main() {
	var processes []*process
	f, err := os.Open("config.json")
	if err != nil {
		log.Fatal(err)
	}
	if err := json.NewDecoder(f).Decode(&processes); err != nil {
		f.Close()
		log.Fatal(err)
	}
	f.Close()

	var (
		s    = supervisor.New()
		sigc = make(chan os.Signal, 1)
	)

	signal.CatchAll(sigc)
	go func() {
		var err error
		for sig := range sigc {
			log.Printf("received signal %v", sig)

			switch sig {
			case syscall.SIGCHLD:
				continue
			case syscall.SIGTERM, syscall.SIGINT:
				err = s.Reap(sig, 10*time.Second)
				os.Exit(0)
			default:
				err = s.Forward(sig)
			}
			if err != nil {
				log.Printf("signal error %s\n", err)
			}
		}
	}()

	for i, p := range processes {
		if err := s.Start(fmt.Sprint(i), p.AttachStd, os.Environ(), nil, p.Args...); err != nil {
			s.Reap(syscall.SIGTERM, 10*time.Second)
			log.Fatal(err)
		}
	}

	if err := s.Wait(); err != nil {
		log.Fatal(err)
	}
}
