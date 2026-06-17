/*
Copyright 2025 Intel Corporation

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
	"context"
	"fmt"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func RegisterOpenTelemetryInstruments(meter metric.Meter) error {
	for resource, features := range GetMonFeatures() {
		switch resource {
		case MonResourceL3:
			for _, f := range features {
				if err := registerInstrument(meter, f); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func registerInstrument(meter metric.Meter, feature string) error {
	instr, err := createInstrument(meter, feature)
	if err != nil {
		return err
	}

	_, err = meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			select {
			case <-ctx.Done():
				return fmt.Errorf("rdt metric collection cancelled: %w", ctx.Err())
			default:
				observeInstrument(o, instr, feature)
			}
			return nil
		},
		instr,
	)
	if err != nil {
		return err
	}

	return nil
}

func createInstrument(meter metric.Meter, feature string) (metric.Int64Observable, error) {
	switch feature {
	case "llc_occupancy":
		name := "l3.llc.occupancy"
		help := "L3 (LLC) occupancy"
		return meter.Int64ObservableGauge(
			name,
			metric.WithDescription(help),
		)

	case "mbm_local_bytes":
		name := "l3.mbm.local"
		help := "bytes transferred to/from local memory through LLC"
		unit := "bytes"
		return meter.Int64ObservableCounter(
			name,
			metric.WithDescription(help),
			metric.WithUnit(unit),
		)

	case "mbm_total_bytes":
		name := "l3.mbm.total"
		help := "total bytes transferred to/from memory through LLC"
		unit := "bytes"
		return meter.Int64ObservableCounter(
			name,
			metric.WithDescription(help),
			metric.WithUnit(unit),
		)
	}

	// an unknown feature, counter for bytes, gauge otherwise
	name := ""
	help := ""
	unit := ""
	if strings.HasSuffix(feature, "_bytes") {
		name = strings.TrimSuffix(feature, "_bytes")
		unit = "bytes"
	}
	name = "l3." + strings.ReplaceAll(name, "_", ".")
	help = "L3 " + feature

	if unit == "bytes" {
		return meter.Int64ObservableUpDownCounter(
			name,
			metric.WithDescription(help),
			metric.WithUnit(unit),
		)
	}

	return meter.Int64ObservableGauge(
		name,
		metric.WithDescription(help),
		metric.WithUnit(unit),
	)
}

func observeInstrument(o metric.Observer, m metric.Int64Observable, feature string) {
	var wg sync.WaitGroup

	for _, cls := range GetClasses() {
		observeGroup(o, m, feature, cls)
		for _, monGrp := range cls.GetMonGroups() {
			wg.Add(1)
			go func(mg MonGroup) {
				defer wg.Done()
				observeGroup(o, m, feature, mg, getMonGroupAttributes(mg)...)
			}(monGrp)
		}
	}

	wg.Wait()
}

func observeGroup(o metric.Observer, m metric.Int64Observable, feature string, g ResctrlGroup, customAttributes ...attribute.KeyValue) {
	var (
		allData = g.GetMonData()
		cgName  string
		mgName  string
	)

	if mg, ok := g.(MonGroup); ok {
		cgName = mg.Parent().Name()
		mgName = mg.Name()
	} else {
		cgName = g.Name()
	}

	attributes := attribute.NewSet(
		attribute.String("rdt.class", cgName),
		attribute.String("rdt.mon.group", mgName),
	)

	for cacheID, data := range allData.L3 {
		if value, ok := data[feature]; ok {
			o.ObserveInt64(
				m,
				int64(value),
				metric.WithAttributeSet(attributes),
				metric.WithAttributes(
					append(
						[]attribute.KeyValue{
							attribute.String("cache.id", fmt.Sprint(cacheID)),
						},
						customAttributes...,
					)...,
				),
			)
		}
	}
}

func getMonGroupAttributes(mg MonGroup) []attribute.KeyValue {
	var (
		annotations = mg.GetAnnotations()
		attributes  = []attribute.KeyValue{}
	)

	for _, name := range customLabels {
		if value, ok := annotations[name]; ok {
			attributes = append(attributes, attribute.String(name, value))
		}
	}

	return attributes
}
