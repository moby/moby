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

package sdkapi // import "go.opentelemetry.io/otel/metric/sdkapi"

import (
	"go.opentelemetry.io/otel/metric/number"
	"go.opentelemetry.io/otel/metric/unit"
)

// Descriptor contains all the settings that describe an instrument,
// including its name, metric kind, number kind, and the configurable
// options.
type Descriptor struct {
	name           string
	instrumentKind InstrumentKind
	numberKind     number.Kind
	description    string
	unit           unit.Unit
}

// NewDescriptor returns a Descriptor with the given contents.
func NewDescriptor(name string, ikind InstrumentKind, nkind number.Kind, description string, unit unit.Unit) Descriptor {
	return Descriptor{
		name:           name,
		instrumentKind: ikind,
		numberKind:     nkind,
		description:    description,
		unit:           unit,
	}
}

// Name returns the metric instrument's name.
func (d Descriptor) Name() string {
	return d.name
}

// InstrumentKind returns the specific kind of instrument.
func (d Descriptor) InstrumentKind() InstrumentKind {
	return d.instrumentKind
}

// Description provides a human-readable description of the metric
// instrument.
func (d Descriptor) Description() string {
	return d.description
}

// Unit describes the units of the metric instrument.  Unitless
// metrics return the empty string.
func (d Descriptor) Unit() unit.Unit {
	return d.unit
}

// NumberKind returns whether this instrument is declared over int64,
// float64, or uint64 values.
func (d Descriptor) NumberKind() number.Kind {
	return d.numberKind
}
