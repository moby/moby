package daemon

import (
	"errors"
	"fmt"
)

// Labels lists known daemon labels and returned.
// This is called directly from the remote API
func (daemon *Daemon) Labels() ([]string, error) {
	return daemon.config().Labels, nil
}

// LabelAdd add a label
// This is called directly from the remote API
func (daemon *Daemon) LabelAdd(name string) ([]string, error) {
	labels := daemon.configStore.Labels
	daemon.configStore.Labels = append(labels, name)
	return daemon.configStore.Labels, nil
}

// LabelRm removes the labels with the given name.
// If the label don't in daemon labels, it is not removed
// This is called directly from the remote API
func (daemon *Daemon) LabelRm(name string) error {
	labels := &daemon.configStore.Labels
	if err := remove(labels, name); err != nil {
		return fmt.Errorf("Error while removing labels %s: %v", name, err)
	}
	return nil
}

func remove(labels *[]string, label string) error {
	for i, elem := range *labels {
		if elem == label {
			*labels = append((*labels)[:i], (*labels)[i+1:]...)
			return nil
		}
	}
	return errors.New("no such label")
}
