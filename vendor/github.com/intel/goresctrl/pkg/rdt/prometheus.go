/*
Copyright 2020 Intel Corporation

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

package rdt

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var customLabels []string = []string{}

// collector implements prometheus.Collector interface
type collector struct {
	descriptors map[string]*prometheus.Desc
}

// NewCollector creates new Prometheus collector of RDT metrics
func NewCollector() prometheus.Collector {
	c := &collector{descriptors: make(map[string]*prometheus.Desc)}
	return c
}

// RegisterCustomPrometheusLabels registers monitor group annotations to be
// exported as Prometheus metrics labels
func RegisterCustomPrometheusLabels(names ...string) {
Names:
	for _, n := range names {
		for _, c := range customLabels {
			if n == c {
				break Names
			}
		}
		customLabels = append(customLabels, n)
	}
}

// Describe method of the prometheus.Collector interface
func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	for resource, features := range GetMonFeatures() {
		switch resource {
		case MonResourceL3:
			for _, f := range features {
				ch <- c.describeL3(f)
			}
		}
	}
}

// Collect method of the prometheus.Collector interface
func (c collector) Collect(ch chan<- prometheus.Metric) {
	var wg sync.WaitGroup

	for _, cls := range GetClasses() {
		c.collectGroupMetrics(ch, cls)
		for _, monGrp := range cls.GetMonGroups() {
			wg.Add(1)
			g := monGrp
			go func() {
				defer wg.Done()
				c.collectMonGroupMetrics(ch, g)
			}()
		}
	}
	wg.Wait()
}

func (c *collector) describeL3(feature string) *prometheus.Desc {
	d, ok := c.descriptors[feature]
	if !ok {
		name := "l3_" + feature
		help := "L3 " + feature

		switch feature {
		case "llc_occupancy":
			help = "L3 (LLC) occupancy"
		case "mbm_local_bytes":
			help = "bytes transferred to/from local memory through LLC"
		case "mbm_total_bytes":
			help = "total bytes transferred to/from memory through LLC"
		}
		labels := append([]string{"rdt_class", "rdt_mon_group", "cache_id"}, customLabels...)
		d = prometheus.NewDesc(name, help, labels, nil)
		c.descriptors[feature] = d
	}
	return d
}

func (c *collector) collectMonGroupMetrics(ch chan<- prometheus.Metric, mg MonGroup) {
	annotations := mg.GetAnnotations()
	customLabelValues := make([]string, len(customLabels))
	for i, name := range customLabels {
		customLabelValues[i] = annotations[name]
	}

	c.collectGroupMetrics(ch, mg, customLabelValues...)
}

func (c *collector) collectGroupMetrics(ch chan<- prometheus.Metric, g ResctrlGroup, customLabels ...string) {
	allData := g.GetMonData()

	cgName, mgName := "", ""
	if mg, ok := g.(MonGroup); ok {
		cgName = mg.Parent().Name()
		mgName = mg.Name()
	} else {
		cgName = g.Name()
	}

	for cacheID, data := range allData.L3 {
		labels := append([]string{cgName, mgName, fmt.Sprint(cacheID)}, customLabels...)
		for feature, value := range data {
			ch <- prometheus.MustNewConstMetric(
				c.describeL3(feature),
				promValueTypeL3(feature),
				float64(value),
				labels...,
			)
		}
	}
}

// promValueTypeL3 returns Prometheus value type for given L3 metric.
func promValueTypeL3(feature string) prometheus.ValueType {
	switch feature {
	case "llc_occupancy":
		return prometheus.GaugeValue
	case "mbm_local_bytes", "mbm_total_bytes":
		return prometheus.CounterValue
	default:
		return prometheus.GaugeValue
	}
}
