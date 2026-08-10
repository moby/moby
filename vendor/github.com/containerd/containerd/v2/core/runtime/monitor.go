/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package runtime

// TaskMonitor provides an interface for monitoring of containers within containerd
type TaskMonitor interface {
	// Monitor adds the provided container to the monitor.
	// Labels are optional (can be nil) key value pairs to be added to the metrics namespace.
	Monitor(task Task, labels map[string]string) error
	// Stop stops and removes the provided container from the monitor
	Stop(task Task) error
}

// NewMultiTaskMonitor returns a new TaskMonitor broadcasting to the provided monitors
func NewMultiTaskMonitor(monitors ...TaskMonitor) TaskMonitor {
	return &multiTaskMonitor{
		monitors: monitors,
	}
}

// NewNoopMonitor is a task monitor that does nothing
func NewNoopMonitor() TaskMonitor {
	return &noopTaskMonitor{}
}

type noopTaskMonitor struct {
}

func (mm *noopTaskMonitor) Monitor(c Task, labels map[string]string) error {
	return nil
}

func (mm *noopTaskMonitor) Stop(c Task) error {
	return nil
}

type multiTaskMonitor struct {
	monitors []TaskMonitor
}

func (mm *multiTaskMonitor) Monitor(task Task, labels map[string]string) error {
	for _, m := range mm.monitors {
		if err := m.Monitor(task, labels); err != nil {
			return err
		}
	}
	return nil
}

func (mm *multiTaskMonitor) Stop(c Task) error {
	for _, m := range mm.monitors {
		if err := m.Stop(c); err != nil {
			return err
		}
	}
	return nil
}
