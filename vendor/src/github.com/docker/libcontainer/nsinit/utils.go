package nsinit

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/docker/libcontainer"
)

func loadContainer() (*libcontainer.Config, error) {
	f, err := os.Open(filepath.Join(dataPath, "container.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var container *libcontainer.Config
	if err := json.NewDecoder(f).Decode(&container); err != nil {
		return nil, err
	}

	return container, nil
}

func openLog(name string) error {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0755)
	if err != nil {
		return err
	}

	log.SetOutput(f)

	return nil
}

func loadContainerFromJson(rawData string) (*libcontainer.Config, error) {
	var container *libcontainer.Config

	if err := json.Unmarshal([]byte(rawData), &container); err != nil {
		return nil, err
	}

	return container, nil
}
