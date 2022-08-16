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

type noopMeterProvider struct{}

// NewNoopMeterProvider returns an implementation of MeterProvider that
// performs no operations. The Meter and Instrument created from the returned
// MeterProvider also perform no operations.
func NewNoopMeterProvider() MeterProvider {
	return noopMeterProvider{}
}

var _ MeterProvider = noopMeterProvider{}

func (noopMeterProvider) Meter(instrumentationName string, opts ...MeterOption) Meter {
	return Meter{}
}
