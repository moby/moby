package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/dotcloud/docker/pkg/libcontainer"
)

func loadContainer() (*libcontainer.Container, error) {
	f, err := os.Open(filepath.Join(dataPath, "container.json"))
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

func readPid() (int, error) {
	data, err := ioutil.ReadFile(filepath.Join(dataPath, "pid"))
	if err != nil {
		return -1, err
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return -1, err
	}

	return pid, nil
}

func openLog(name string) error {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0755)
	if err != nil {
		return err
	}

	log.SetOutput(f)

	return nil
}

func loadContainerFromJson(rawData string) (*libcontainer.Container, error) {
	var container *libcontainer.Container

	if err := json.Unmarshal([]byte(rawData), &container); err != nil {
		return nil, err
	}

	return container, nil
}
