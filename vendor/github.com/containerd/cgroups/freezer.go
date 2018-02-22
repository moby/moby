package cgroups

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"
)

func NewFreezer(root string) *freezerController {
	return &freezerController{
		root: filepath.Join(root, string(Freezer)),
	}
}

type freezerController struct {
	root string
}

func (f *freezerController) Name() Name {
	return Freezer
}

func (f *freezerController) Path(path string) string {
	return filepath.Join(f.root, path)
}

func (f *freezerController) Freeze(path string) error {
	return f.waitState(path, Frozen)
}

func (f *freezerController) Thaw(path string) error {
	return f.waitState(path, Thawed)
}

func (f *freezerController) changeState(path string, state State) error {
	return ioutil.WriteFile(
		filepath.Join(f.root, path, "freezer.state"),
		[]byte(strings.ToUpper(string(state))),
		defaultFilePerm,
	)
}

func (f *freezerController) state(path string) (State, error) {
	current, err := ioutil.ReadFile(filepath.Join(f.root, path, "freezer.state"))
	if err != nil {
		return "", err
	}
	return State(strings.ToLower(strings.TrimSpace(string(current)))), nil
}

func (f *freezerController) waitState(path string, state State) error {
	for {
		if err := f.changeState(path, state); err != nil {
			return err
		}
		current, err := f.state(path)
		if err != nil {
			return err
		}
		if current == state {
			return nil
		}
		time.Sleep(1 * time.Millisecond)
	}
}
