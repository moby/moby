package main

import (
	"encoding/json"
	"errors"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"log"
	"os"
)

var (
	ErrUnsupported    = errors.New("Unsupported method")
	ErrWrongArguments = errors.New("Wrong argument count")
)

func main() {
	container, err := loadContainer()
	if err != nil {
		log.Fatal(err)
	}

	argc := len(os.Args)
	if argc < 2 {
		log.Fatal(ErrWrongArguments)
	}
	switch os.Args[1] {
	case "exec":
		exitCode, err := execCommand(container)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(exitCode)
	case "init":
		if argc != 3 {
			log.Fatal(ErrWrongArguments)
		}
		if err := initCommand(container, os.Args[2]); err != nil {
			log.Fatal(err)
		}
	}
}

func loadContainer() (*libcontainer.Container, error) {
	f, err := os.Open("container.json")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var container *libcontainer.Container
	if err := json.NewDecoder(f).Decode(&container); err != nil {
		return nil, err
	}
	return container, nil
}
