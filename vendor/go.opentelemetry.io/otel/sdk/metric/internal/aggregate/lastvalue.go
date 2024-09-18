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

package aggregate // import "go.opentelemetry.io/otel/sdk/metric/internal/aggregate"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// datapoint is timestamped measurement data.
type datapoint[N int64 | float64] struct {
	timestamp time.Time
	value     N
}

func newLastValue[N int64 | float64]() *lastValue[N] {
	return &lastValue[N]{values: make(map[attribute.Set]datapoint[N])}
}

// lastValue summarizes a set of measurements as the last one made.
type lastValue[N int64 | float64] struct {
	sync.Mutex

	values map[attribute.Set]datapoint[N]
}

func (s *lastValue[N]) measure(ctx context.Context, value N, attr attribute.Set) {
	d := datapoint[N]{timestamp: now(), value: value}
	s.Lock()
	s.values[attr] = d
	s.Unlock()
}

func (s *lastValue[N]) computeAggregation(dest *[]metricdata.DataPoint[N]) {
	s.Lock()
	defer s.Unlock()

	n := len(s.values)
	*dest = reset(*dest, n, n)

	var i int
	for a, v := range s.values {
		(*dest)[i].Attributes = a
		// The event time is the only meaningful timestamp, StartTime is
		// ignored.
		(*dest)[i].Time = v.timestamp
		(*dest)[i].Value = v.value
		// Do not report stale values.
		delete(s.values, a)
		i++
	}
}
