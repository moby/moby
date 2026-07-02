//go:build linux

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

package v2

import (
	"context"

	"github.com/containerd/containerd/v2/core/events"
	"github.com/containerd/containerd/v2/core/runtime"
	"github.com/docker/go-metrics"
)

// NewTaskMonitor returns a new cgroups monitor
func NewTaskMonitor(ctx context.Context, publisher events.Publisher, ns *metrics.Namespace) (runtime.TaskMonitor, error) {
	collector := NewCollector(ns)
	return &cgroupsMonitor{
		collector: collector,
		context:   ctx,
		publisher: publisher,
	}, nil
}

type cgroupsMonitor struct {
	collector *Collector
	context   context.Context
	publisher events.Publisher
}

func (m *cgroupsMonitor) Monitor(c runtime.Task, labels map[string]string) error {
	if err := m.collector.Add(c, labels); err != nil {
		return err
	}
	return nil
}

func (m *cgroupsMonitor) Stop(c runtime.Task) error {
	m.collector.Remove(c)
	return nil
}
