// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metric // import "go.opentelemetry.io/otel/metric"

import (
	"go.opentelemetry.io/otel/metric/unit"
)

// InstrumentConfig contains options for metric instrument descriptors.
type InstrumentConfig struct {
	description            string
	unit                   unit.Unit
	instrumentationName    string
	instrumentationVersion string
}

// Description describes the instrument in human-readable terms.
func (cfg InstrumentConfig) Description() string {
	return cfg.description
}

// Unit describes the measurement unit for a instrument.
func (cfg InstrumentConfig) Unit() unit.Unit {
	return cfg.unit
}

// InstrumentationName is the name of the library providing
// instrumentation.
func (cfg InstrumentConfig) InstrumentationName() string {
	return cfg.instrumentationName
}

// InstrumentationVersion is the version of the library providing
// instrumentation.
func (cfg InstrumentConfig) InstrumentationVersion() string {
	return cfg.instrumentationVersion
}

// InstrumentOption is an interface for applying metric instrument options.
type InstrumentOption interface {
	// ApplyMeter is used to set a InstrumentOption value of a
	// InstrumentConfig.
	applyInstrument(*InstrumentConfig)
}

// NewInstrumentConfig creates a new InstrumentConfig
// and applies all the given options.
func NewInstrumentConfig(opts ...InstrumentOption) InstrumentConfig {
	var config InstrumentConfig
	for _, o := range opts {
		o.applyInstrument(&config)
	}
	return config
}

type instrumentOptionFunc func(*InstrumentConfig)

func (fn instrumentOptionFunc) applyInstrument(cfg *InstrumentConfig) {
	fn(cfg)
}

// WithDescription applies provided description.
func WithDescription(desc string) InstrumentOption {
	return instrumentOptionFunc(func(cfg *InstrumentConfig) {
		cfg.description = desc
	})
}

// WithUnit applies provided unit.
func WithUnit(unit unit.Unit) InstrumentOption {
	return instrumentOptionFunc(func(cfg *InstrumentConfig) {
		cfg.unit = unit
	})
}

// WithInstrumentationName sets the instrumentation name.
func WithInstrumentationName(name string) InstrumentOption {
	return instrumentOptionFunc(func(cfg *InstrumentConfig) {
		cfg.instrumentationName = name
	})
}

// MeterConfig contains options for Meters.
type MeterConfig struct {
	instrumentationVersion string
}

// InstrumentationVersion is the version of the library providing instrumentation.
func (cfg MeterConfig) InstrumentationVersion() string {
	return cfg.instrumentationVersion
}

// MeterOption is an interface for applying Meter options.
type MeterOption interface {
	// ApplyMeter is used to set a MeterOption value of a MeterConfig.
	applyMeter(*MeterConfig)
}

// NewMeterConfig creates a new MeterConfig and applies
// all the given options.
func NewMeterConfig(opts ...MeterOption) MeterConfig {
	var config MeterConfig
	for _, o := range opts {
		o.applyMeter(&config)
	}
	return config
}

// InstrumentMeterOption are options that can be used as both an InstrumentOption
// and MeterOption
type InstrumentMeterOption interface {
	InstrumentOption
	MeterOption
}

// WithInstrumentationVersion sets the instrumentation version.
func WithInstrumentationVersion(version string) InstrumentMeterOption {
	return instrumentationVersionOption(version)
}

type instrumentationVersionOption string

func (i instrumentationVersionOption) applyMeter(config *MeterConfig) {
	config.instrumentationVersion = string(i)
}

func (i instrumentationVersionOption) applyInstrument(config *InstrumentConfig) {
	config.instrumentationVersion = string(i)
}
