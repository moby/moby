package daemon

import (
	"fmt"
	"strings"
)

// ModifyLabel modify daemon label
func (daemon *Daemon) ModifyLabel(method, value string) error {
	if strings.Count(value, "=") != 1 {
		return fmt.Errorf("Bad format label '%s'", value)
	}
	if method == "Add" {
		labels := daemon.configStore.Labels
		for _, label := range labels {
			if label == value {
				return nil
			}
		}
		daemon.configStore.Labels = append(labels, value)
	}
	if method == "Remove" {
		labels := daemon.configStore.Labels
		for i, label := range labels {
			if label == value {
				daemon.configStore.Labels = append(labels[0:i], labels[i+1:]...)
				return nil
			}
		}
		return fmt.Errorf("Remove failed: lable '%s' is not exists", value)
	}
	if method == "Modify" {
		key := strings.Split(value, "=")[0]
		labels := daemon.configStore.Labels
		for i, label := range labels {
			if key == strings.Split(label, "=")[0] {
				daemon.configStore.Labels[i] = value
				return nil
			}
		}
	}
	return nil
}
