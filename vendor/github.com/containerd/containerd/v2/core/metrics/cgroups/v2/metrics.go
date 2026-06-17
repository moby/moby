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
	"fmt"
	"sync"

	cmetrics "github.com/containerd/containerd/v2/core/metrics"
	"github.com/containerd/containerd/v2/core/metrics/cgroups/common"
	v2 "github.com/containerd/containerd/v2/core/metrics/types/v2"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/timeout"
	"github.com/containerd/log"
	"github.com/containerd/typeurl/v2"
	"github.com/docker/go-metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// NewCollector registers the collector with the provided namespace and returns it so
// that cgroups can be added for collection
func NewCollector(ns *metrics.Namespace) *Collector {
	if ns == nil {
		return &Collector{}
	}
	c := &Collector{
		ns:    ns,
		tasks: make(map[string]entry),
	}
	c.metrics = append(c.metrics, pidMetrics...)
	c.metrics = append(c.metrics, cpuMetrics...)
	c.metrics = append(c.metrics, memoryMetrics...)
	c.metrics = append(c.metrics, ioMetrics...)
	c.storedMetrics = make(chan prometheus.Metric, 100*len(c.metrics))
	ns.Add(c)
	return c
}

func taskID(id, namespace string) string {
	return fmt.Sprintf("%s-%s", id, namespace)
}

type entry struct {
	task common.Statable
	// ns is an optional child namespace that contains additional to parent labels.
	// This can be used to append task specific labels to be able to differentiate the different containerd metrics.
	ns *metrics.Namespace
}

// Collector provides the ability to collect container stats and export
// them in the prometheus format
type Collector struct {
	ns            *metrics.Namespace
	storedMetrics chan prometheus.Metric

	// TODO(fuweid):
	//
	// The Collector.Collect will be the field ns'Collect's callback,
	// which be invoked periodically with internal lock. And Collector.Add
	// might also invoke ns.Lock if the labels is not nil, which is easy to
	// cause dead-lock.
	//
	// Goroutine X:
	//
	//	ns.Collect
	//   	  ns.Lock
	//          Collector.Collect
	//            Collector.RLock
	//
	//
	// Goroutine Y:
	//
	//	Collector.Add
	//        ...(RLock/Lock)
	//	    ns.Lock
	//
	// I think we should seek the way to decouple ns from Collector.
	mu      sync.RWMutex
	tasks   map[string]entry
	metrics []*metric
}

// Describe prometheus metrics
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range c.metrics {
		ch <- m.desc(c.ns)
	}
}

// Collect prometheus metrics
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	wg := &sync.WaitGroup{}
	for _, t := range c.tasks {
		wg.Add(1)
		go c.collect(t, ch, true, wg)
	}
storedLoop:
	for {
		// read stored metrics until the channel is flushed
		select {
		case m := <-c.storedMetrics:
			ch <- m
		default:
			break storedLoop
		}
	}
	c.mu.RUnlock()
	wg.Wait()
}

func (c *Collector) collect(entry entry, ch chan<- prometheus.Metric, block bool, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}

	t := entry.task
	ctx, cancel := timeout.WithContext(context.Background(), cmetrics.ShimStatsRequestTimeout)
	stats, err := t.Stats(namespaces.WithNamespace(ctx, t.Namespace()))
	cancel()

	if err != nil {
		log.L.WithError(err).Errorf("stat task %s", t.ID())
		return
	}

	s := &v2.Metrics{}
	if err := typeurl.UnmarshalTo(stats, s); err != nil {
		log.L.WithError(err).Errorf("unmarshal stats for %s", t.ID())
		return
	}
	ns := entry.ns
	if ns == nil {
		ns = c.ns
	}
	for _, m := range c.metrics {
		m.collect(t.ID(), t.Namespace(), s, ns, ch, block)
	}
}

// Add adds the provided cgroup and id so that metrics are collected and exported
func (c *Collector) Add(t common.Statable, labels map[string]string) error {
	if c.ns == nil {
		return nil
	}
	c.mu.RLock()
	id := taskID(t.ID(), t.Namespace())
	_, ok := c.tasks[id]
	c.mu.RUnlock()
	if ok {
		return nil // requests to collect metrics should be idempotent
	}
	entry := entry{task: t}
	if labels != nil {
		entry.ns = c.ns.WithConstLabels(labels)
	}
	c.mu.Lock()
	c.tasks[id] = entry
	c.mu.Unlock()
	return nil
}

// Remove removes the provided cgroup by id from the collector
func (c *Collector) Remove(t common.Statable) {
	if c.ns == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.tasks, taskID(t.ID(), t.Namespace()))
}

// RemoveAll statable items from the collector
func (c *Collector) RemoveAll() {
	if c.ns == nil {
		return
	}
	c.mu.Lock()
	c.tasks = make(map[string]entry)
	c.mu.Unlock()
}
