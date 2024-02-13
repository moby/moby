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

package global // import "go.opentelemetry.io/otel/internal/global"

import (
	"context"
	"sync/atomic"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
)

// unwrapper unwraps to return the underlying instrument implementation.
type unwrapper interface {
	Unwrap() metric.Observable
}

type afCounter struct {
	embedded.Float64ObservableCounter
	metric.Float64Observable

	name string
	opts []metric.Float64ObservableCounterOption

	delegate atomic.Value //metric.Float64ObservableCounter
}

var _ unwrapper = (*afCounter)(nil)
var _ metric.Float64ObservableCounter = (*afCounter)(nil)

func (i *afCounter) setDelegate(m metric.Meter) {
	ctr, err := m.Float64ObservableCounter(i.name, i.opts...)
	if err != nil {
		GetErrorHandler().Handle(err)
		return
	}
	i.delegate.Store(ctr)
}

func (i *afCounter) Unwrap() metric.Observable {
	if ctr := i.delegate.Load(); ctr != nil {
		return ctr.(metric.Float64ObservableCounter)
	}
	return nil
}

type afUpDownCounter struct {
	embedded.Float64ObservableUpDownCounter
	metric.Float64Observable

	name string
	opts []metric.Float64ObservableUpDownCounterOption

	delegate atomic.Value //metric.Float64ObservableUpDownCounter
}

var _ unwrapper = (*afUpDownCounter)(nil)
var _ metric.Float64ObservableUpDownCounter = (*afUpDownCounter)(nil)

func (i *afUpDownCounter) setDelegate(m metric.Meter) {
	ctr, err := m.Float64ObservableUpDownCounter(i.name, i.opts...)
	if err != nil {
		GetErrorHandler().Handle(err)
		return
	}
	i.delegate.Store(ctr)
}

func (i *afUpDownCounter) Unwrap() metric.Observable {
	if ctr := i.delegate.Load(); ctr != nil {
		return ctr.(metric.Float64ObservableUpDownCounter)
	}
	return nil
}

type afGauge struct {
	embedded.Float64ObservableGauge
	metric.Float64Observable

	name string
	opts []metric.Float64ObservableGaugeOption

	delegate atomic.Value //metric.Float64ObservableGauge
}

var _ unwrapper = (*afGauge)(nil)
var _ metric.Float64ObservableGauge = (*afGauge)(nil)

func (i *afGauge) setDelegate(m metric.Meter) {
	ctr, err := m.Float64ObservableGauge(i.name, i.opts...)
	if err != nil {
		GetErrorHandler().Handle(err)
		return
	}
	i.delegate.Store(ctr)
}

func (i *afGauge) Unwrap() metric.Observable {
	if ctr := i.delegate.Load(); ctr != nil {
		return ctr.(metric.Float64ObservableGauge)
	}
	return nil
}

type aiCounter struct {
	embedded.Int64ObservableCounter
	metric.Int64Observable

	name string
	opts []metric.Int64ObservableCounterOption

	delegate atomic.Value //metric.Int64ObservableCounter
}

var _ unwrapper = (*aiCounter)(nil)
var _ metric.Int64ObservableCounter = (*aiCounter)(nil)

func (i *aiCounter) setDelegate(m metric.Meter) {
	ctr, err := m.Int64ObservableCounter(i.name, i.opts...)
	if err != nil {
		GetErrorHandler().Handle(err)
		return
	}
	i.delegate.Store(ctr)
}

func (i *aiCounter) Unwrap() metric.Observable {
	if ctr := i.delegate.Load(); ctr != nil {
		return ctr.(metric.Int64ObservableCounter)
	}
	return nil
}

type aiUpDownCounter struct {
	embedded.Int64ObservableUpDownCounter
	metric.Int64Observable

	name string
	opts []metric.Int64ObservableUpDownCounterOption

	delegate atomic.Value //metric.Int64ObservableUpDownCounter
}

var _ unwrapper = (*aiUpDownCounter)(nil)
var _ metric.Int64ObservableUpDownCounter = (*aiUpDownCounter)(nil)

func (i *aiUpDownCounter) setDelegate(m metric.Meter) {
	ctr, err := m.Int64ObservableUpDownCounter(i.name, i.opts...)
	if err != nil {
		GetErrorHandler().Handle(err)
		return
	}
	i.delegate.Store(ctr)
}

func (i *aiUpDownCounter) Unwrap() metric.Observable {
	if ctr := i.delegate.Load(); ctr != nil {
		return ctr.(metric.Int64ObservableUpDownCounter)
	}
	return nil
}

type aiGauge struct {
	embedded.Int64ObservableGauge
	metric.Int64Observable

	name string
	opts []metric.Int64ObservableGaugeOption

	delegate atomic.Value //metric.Int64ObservableGauge
}

var _ unwrapper = (*aiGauge)(nil)
var _ metric.Int64ObservableGauge = (*aiGauge)(nil)

func (i *aiGauge) setDelegate(m metric.Meter) {
	ctr, err := m.Int64ObservableGauge(i.name, i.opts...)
	if err != nil {
		GetErrorHandler().Handle(err)
		return
	}
	i.delegate.Store(ctr)
}

func (i *aiGauge) Unwrap() metric.Observable {
	if ctr := i.delegate.Load(); ctr != nil {
		return ctr.(metric.Int64ObservableGauge)
	}
	return nil
}

// Sync Instruments.
type sfCounter struct {
	embedded.Float64Counter

	name string
	opts []metric.Float64CounterOption

	delegate atomic.Value //metric.Float64Counter
}

var _ metric.Float64Counter = (*sfCounter)(nil)

func (i *sfCounter) setDelegate(m metric.Meter) {
	ctr, err := m.Float64Counter(i.name, i.opts...)
	if err != nil {
		GetErrorHandler().Handle(err)
		return
	}
	i.delegate.Store(ctr)
}

func (i *sfCounter) Add(ctx context.Context, incr float64, opts ...metric.AddOption) {
	if ctr := i.delegate.Load(); ctr != nil {
		ctr.(metric.Float64Counter).Add(ctx, incr, opts...)
	}
}

type sfUpDownCounter struct {
	embedded.Float64UpDownCounter

	name string
	opts []metric.Float64UpDownCounterOption

	delegate atomic.Value //metric.Float64UpDownCounter
}

var _ metric.Float64UpDownCounter = (*sfUpDownCounter)(nil)

func (i *sfUpDownCounter) setDelegate(m metric.Meter) {
	ctr, err := m.Float64UpDownCounter(i.name, i.opts...)
	if err != nil {
		GetErrorHandler().Handle(err)
		return
	}
	i.delegate.Store(ctr)
}

func (i *sfUpDownCounter) Add(ctx context.Context, incr float64, opts ...metric.AddOption) {
	if ctr := i.delegate.Load(); ctr != nil {
		ctr.(metric.Float64UpDownCounter).Add(ctx, incr, opts...)
	}
}

type sfHistogram struct {
	embedded.Float64Histogram

	name string
	opts []metric.Float64HistogramOption

	delegate atomic.Value //metric.Float64Histogram
}

var _ metric.Float64Histogram = (*sfHistogram)(nil)

func (i *sfHistogram) setDelegate(m metric.Meter) {
	ctr, err := m.Float64Histogram(i.name, i.opts...)
	if err != nil {
		GetErrorHandler().Handle(err)
		return
	}
	i.delegate.Store(ctr)
}

func (i *sfHistogram) Record(ctx context.Context, x float64, opts ...metric.RecordOption) {
	if ctr := i.delegate.Load(); ctr != nil {
		ctr.(metric.Float64Histogram).Record(ctx, x, opts...)
	}
}

type siCounter struct {
	embedded.Int64Counter

	name string
	opts []metric.Int64CounterOption

	delegate atomic.Value //metric.Int64Counter
}

var _ metric.Int64Counter = (*siCounter)(nil)

func (i *siCounter) setDelegate(m metric.Meter) {
	ctr, err := m.Int64Counter(i.name, i.opts...)
	if err != nil {
		GetErrorHandler().Handle(err)
		return
	}
	i.delegate.Store(ctr)
}

func (i *siCounter) Add(ctx context.Context, x int64, opts ...metric.AddOption) {
	if ctr := i.delegate.Load(); ctr != nil {
		ctr.(metric.Int64Counter).Add(ctx, x, opts...)
	}
}

type siUpDownCounter struct {
	embedded.Int64UpDownCounter

	name string
	opts []metric.Int64UpDownCounterOption

	delegate atomic.Value //metric.Int64UpDownCounter
}

var _ metric.Int64UpDownCounter = (*siUpDownCounter)(nil)

func (i *siUpDownCounter) setDelegate(m metric.Meter) {
	ctr, err := m.Int64UpDownCounter(i.name, i.opts...)
	if err != nil {
		GetErrorHandler().Handle(err)
		return
	}
	i.delegate.Store(ctr)
}

func (i *siUpDownCounter) Add(ctx context.Context, x int64, opts ...metric.AddOption) {
	if ctr := i.delegate.Load(); ctr != nil {
		ctr.(metric.Int64UpDownCounter).Add(ctx, x, opts...)
	}
}

type siHistogram struct {
	embedded.Int64Histogram

	name string
	opts []metric.Int64HistogramOption

	delegate atomic.Value //metric.Int64Histogram
}

var _ metric.Int64Histogram = (*siHistogram)(nil)

func (i *siHistogram) setDelegate(m metric.Meter) {
	ctr, err := m.Int64Histogram(i.name, i.opts...)
	if err != nil {
		GetErrorHandler().Handle(err)
		return
	}
	i.delegate.Store(ctr)
}

func (i *siHistogram) Record(ctx context.Context, x int64, opts ...metric.RecordOption) {
	if ctr := i.delegate.Load(); ctr != nil {
		ctr.(metric.Int64Histogram).Record(ctx, x, opts...)
	}
}
