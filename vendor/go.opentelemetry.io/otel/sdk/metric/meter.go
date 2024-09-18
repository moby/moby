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

package metric // import "go.opentelemetry.io/otel/sdk/metric"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/internal/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/internal/aggregate"
)

// ErrInstrumentName indicates the created instrument has an invalid name.
// Valid names must consist of 255 or fewer characters including alphanumeric, _, ., -, / and start with a letter.
var ErrInstrumentName = errors.New("invalid instrument name")

// meter handles the creation and coordination of all metric instruments. A
// meter represents a single instrumentation scope; all metric telemetry
// produced by an instrumentation scope will use metric instruments from a
// single meter.
type meter struct {
	embedded.Meter

	scope instrumentation.Scope
	pipes pipelines

	int64Resolver   resolver[int64]
	float64Resolver resolver[float64]
}

func newMeter(s instrumentation.Scope, p pipelines) *meter {
	// viewCache ensures instrument conflicts, including number conflicts, this
	// meter is asked to create are logged to the user.
	var viewCache cache[string, instID]

	return &meter{
		scope:           s,
		pipes:           p,
		int64Resolver:   newResolver[int64](p, &viewCache),
		float64Resolver: newResolver[float64](p, &viewCache),
	}
}

// Compile-time check meter implements metric.Meter.
var _ metric.Meter = (*meter)(nil)

// Int64Counter returns a new instrument identified by name and configured with
// options. The instrument is used to synchronously record increasing int64
// measurements during a computational operation.
func (m *meter) Int64Counter(name string, options ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	cfg := metric.NewInt64CounterConfig(options...)
	const kind = InstrumentKindCounter
	p := int64InstProvider{m}
	i, err := p.lookup(kind, name, cfg.Description(), cfg.Unit())
	if err != nil {
		return i, err
	}

	return i, validateInstrumentName(name)
}

// Int64UpDownCounter returns a new instrument identified by name and
// configured with options. The instrument is used to synchronously record
// int64 measurements during a computational operation.
func (m *meter) Int64UpDownCounter(name string, options ...metric.Int64UpDownCounterOption) (metric.Int64UpDownCounter, error) {
	cfg := metric.NewInt64UpDownCounterConfig(options...)
	const kind = InstrumentKindUpDownCounter
	p := int64InstProvider{m}
	i, err := p.lookup(kind, name, cfg.Description(), cfg.Unit())
	if err != nil {
		return i, err
	}

	return i, validateInstrumentName(name)
}

// Int64Histogram returns a new instrument identified by name and configured
// with options. The instrument is used to synchronously record the
// distribution of int64 measurements during a computational operation.
func (m *meter) Int64Histogram(name string, options ...metric.Int64HistogramOption) (metric.Int64Histogram, error) {
	cfg := metric.NewInt64HistogramConfig(options...)
	p := int64InstProvider{m}
	i, err := p.lookupHistogram(name, cfg)
	if err != nil {
		return i, err
	}

	return i, validateInstrumentName(name)
}

// Int64ObservableCounter returns a new instrument identified by name and
// configured with options. The instrument is used to asynchronously record
// increasing int64 measurements once per a measurement collection cycle.
// Only the measurements recorded during the collection cycle are exported.
func (m *meter) Int64ObservableCounter(name string, options ...metric.Int64ObservableCounterOption) (metric.Int64ObservableCounter, error) {
	cfg := metric.NewInt64ObservableCounterConfig(options...)
	const kind = InstrumentKindObservableCounter
	p := int64ObservProvider{m}
	inst, err := p.lookup(kind, name, cfg.Description(), cfg.Unit())
	if err != nil {
		return nil, err
	}
	p.registerCallbacks(inst, cfg.Callbacks())
	return inst, validateInstrumentName(name)
}

// Int64ObservableUpDownCounter returns a new instrument identified by name and
// configured with options. The instrument is used to asynchronously record
// int64 measurements once per a measurement collection cycle. Only the
// measurements recorded during the collection cycle are exported.
func (m *meter) Int64ObservableUpDownCounter(name string, options ...metric.Int64ObservableUpDownCounterOption) (metric.Int64ObservableUpDownCounter, error) {
	cfg := metric.NewInt64ObservableUpDownCounterConfig(options...)
	const kind = InstrumentKindObservableUpDownCounter
	p := int64ObservProvider{m}
	inst, err := p.lookup(kind, name, cfg.Description(), cfg.Unit())
	if err != nil {
		return nil, err
	}
	p.registerCallbacks(inst, cfg.Callbacks())
	return inst, validateInstrumentName(name)
}

// Int64ObservableGauge returns a new instrument identified by name and
// configured with options. The instrument is used to asynchronously record
// instantaneous int64 measurements once per a measurement collection cycle.
// Only the measurements recorded during the collection cycle are exported.
func (m *meter) Int64ObservableGauge(name string, options ...metric.Int64ObservableGaugeOption) (metric.Int64ObservableGauge, error) {
	cfg := metric.NewInt64ObservableGaugeConfig(options...)
	const kind = InstrumentKindObservableGauge
	p := int64ObservProvider{m}
	inst, err := p.lookup(kind, name, cfg.Description(), cfg.Unit())
	if err != nil {
		return nil, err
	}
	p.registerCallbacks(inst, cfg.Callbacks())
	return inst, validateInstrumentName(name)
}

// Float64Counter returns a new instrument identified by name and configured
// with options. The instrument is used to synchronously record increasing
// float64 measurements during a computational operation.
func (m *meter) Float64Counter(name string, options ...metric.Float64CounterOption) (metric.Float64Counter, error) {
	cfg := metric.NewFloat64CounterConfig(options...)
	const kind = InstrumentKindCounter
	p := float64InstProvider{m}
	i, err := p.lookup(kind, name, cfg.Description(), cfg.Unit())
	if err != nil {
		return i, err
	}

	return i, validateInstrumentName(name)
}

// Float64UpDownCounter returns a new instrument identified by name and
// configured with options. The instrument is used to synchronously record
// float64 measurements during a computational operation.
func (m *meter) Float64UpDownCounter(name string, options ...metric.Float64UpDownCounterOption) (metric.Float64UpDownCounter, error) {
	cfg := metric.NewFloat64UpDownCounterConfig(options...)
	const kind = InstrumentKindUpDownCounter
	p := float64InstProvider{m}
	i, err := p.lookup(kind, name, cfg.Description(), cfg.Unit())
	if err != nil {
		return i, err
	}

	return i, validateInstrumentName(name)
}

// Float64Histogram returns a new instrument identified by name and configured
// with options. The instrument is used to synchronously record the
// distribution of float64 measurements during a computational operation.
func (m *meter) Float64Histogram(name string, options ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	cfg := metric.NewFloat64HistogramConfig(options...)
	p := float64InstProvider{m}
	i, err := p.lookupHistogram(name, cfg)
	if err != nil {
		return i, err
	}

	return i, validateInstrumentName(name)
}

// Float64ObservableCounter returns a new instrument identified by name and
// configured with options. The instrument is used to asynchronously record
// increasing float64 measurements once per a measurement collection cycle.
// Only the measurements recorded during the collection cycle are exported.
func (m *meter) Float64ObservableCounter(name string, options ...metric.Float64ObservableCounterOption) (metric.Float64ObservableCounter, error) {
	cfg := metric.NewFloat64ObservableCounterConfig(options...)
	const kind = InstrumentKindObservableCounter
	p := float64ObservProvider{m}
	inst, err := p.lookup(kind, name, cfg.Description(), cfg.Unit())
	if err != nil {
		return nil, err
	}
	p.registerCallbacks(inst, cfg.Callbacks())
	return inst, validateInstrumentName(name)
}

// Float64ObservableUpDownCounter returns a new instrument identified by name
// and configured with options. The instrument is used to asynchronously record
// float64 measurements once per a measurement collection cycle. Only the
// measurements recorded during the collection cycle are exported.
func (m *meter) Float64ObservableUpDownCounter(name string, options ...metric.Float64ObservableUpDownCounterOption) (metric.Float64ObservableUpDownCounter, error) {
	cfg := metric.NewFloat64ObservableUpDownCounterConfig(options...)
	const kind = InstrumentKindObservableUpDownCounter
	p := float64ObservProvider{m}
	inst, err := p.lookup(kind, name, cfg.Description(), cfg.Unit())
	if err != nil {
		return nil, err
	}
	p.registerCallbacks(inst, cfg.Callbacks())
	return inst, validateInstrumentName(name)
}

// Float64ObservableGauge returns a new instrument identified by name and
// configured with options. The instrument is used to asynchronously record
// instantaneous float64 measurements once per a measurement collection cycle.
// Only the measurements recorded during the collection cycle are exported.
func (m *meter) Float64ObservableGauge(name string, options ...metric.Float64ObservableGaugeOption) (metric.Float64ObservableGauge, error) {
	cfg := metric.NewFloat64ObservableGaugeConfig(options...)
	const kind = InstrumentKindObservableGauge
	p := float64ObservProvider{m}
	inst, err := p.lookup(kind, name, cfg.Description(), cfg.Unit())
	if err != nil {
		return nil, err
	}
	p.registerCallbacks(inst, cfg.Callbacks())
	return inst, validateInstrumentName(name)
}

func validateInstrumentName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("%w: %s: is empty", ErrInstrumentName, name)
	}
	if len(name) > 255 {
		return fmt.Errorf("%w: %s: longer than 255 characters", ErrInstrumentName, name)
	}
	if !isAlpha([]rune(name)[0]) {
		return fmt.Errorf("%w: %s: must start with a letter", ErrInstrumentName, name)
	}
	if len(name) == 1 {
		return nil
	}
	for _, c := range name[1:] {
		if !isAlphanumeric(c) && c != '_' && c != '.' && c != '-' && c != '/' {
			return fmt.Errorf("%w: %s: must only contain [A-Za-z0-9_.-/]", ErrInstrumentName, name)
		}
	}
	return nil
}

func isAlpha(c rune) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

func isAlphanumeric(c rune) bool {
	return isAlpha(c) || ('0' <= c && c <= '9')
}

// RegisterCallback registers f to be called each collection cycle so it will
// make observations for insts during those cycles.
//
// The only instruments f can make observations for are insts. All other
// observations will be dropped and an error will be logged.
//
// Only instruments from this meter can be registered with f, an error is
// returned if other instrument are provided.
//
// Only observations made in the callback will be exported. Unlike synchronous
// instruments, asynchronous callbacks can "forget" attribute sets that are no
// longer relevant by omitting the observation during the callback.
//
// The returned Registration can be used to unregister f.
func (m *meter) RegisterCallback(f metric.Callback, insts ...metric.Observable) (metric.Registration, error) {
	if len(insts) == 0 {
		// Don't allocate a observer if not needed.
		return noopRegister{}, nil
	}

	reg := newObserver()
	var errs multierror
	for _, inst := range insts {
		// Unwrap any global.
		if u, ok := inst.(interface {
			Unwrap() metric.Observable
		}); ok {
			inst = u.Unwrap()
		}

		switch o := inst.(type) {
		case int64Observable:
			if err := o.registerable(m); err != nil {
				if !errors.Is(err, errEmptyAgg) {
					errs.append(err)
				}
				continue
			}
			reg.registerInt64(o.observablID)
		case float64Observable:
			if err := o.registerable(m); err != nil {
				if !errors.Is(err, errEmptyAgg) {
					errs.append(err)
				}
				continue
			}
			reg.registerFloat64(o.observablID)
		default:
			// Instrument external to the SDK.
			return nil, fmt.Errorf("invalid observable: from different implementation")
		}
	}

	err := errs.errorOrNil()
	if reg.len() == 0 {
		// All insts use drop aggregation or are invalid.
		return noopRegister{}, err
	}

	// Some or all instruments were valid.
	cback := func(ctx context.Context) error { return f(ctx, reg) }
	return m.pipes.registerMultiCallback(cback), err
}

type observer struct {
	embedded.Observer

	float64 map[observablID[float64]]struct{}
	int64   map[observablID[int64]]struct{}
}

func newObserver() observer {
	return observer{
		float64: make(map[observablID[float64]]struct{}),
		int64:   make(map[observablID[int64]]struct{}),
	}
}

func (r observer) len() int {
	return len(r.float64) + len(r.int64)
}

func (r observer) registerFloat64(id observablID[float64]) {
	r.float64[id] = struct{}{}
}

func (r observer) registerInt64(id observablID[int64]) {
	r.int64[id] = struct{}{}
}

var (
	errUnknownObserver = errors.New("unknown observable instrument")
	errUnregObserver   = errors.New("observable instrument not registered for callback")
)

func (r observer) ObserveFloat64(o metric.Float64Observable, v float64, opts ...metric.ObserveOption) {
	var oImpl float64Observable
	switch conv := o.(type) {
	case float64Observable:
		oImpl = conv
	case interface {
		Unwrap() metric.Observable
	}:
		// Unwrap any global.
		async := conv.Unwrap()
		var ok bool
		if oImpl, ok = async.(float64Observable); !ok {
			global.Error(errUnknownObserver, "failed to record asynchronous")
			return
		}
	default:
		global.Error(errUnknownObserver, "failed to record")
		return
	}

	if _, registered := r.float64[oImpl.observablID]; !registered {
		global.Error(errUnregObserver, "failed to record",
			"name", oImpl.name,
			"description", oImpl.description,
			"unit", oImpl.unit,
			"number", fmt.Sprintf("%T", float64(0)),
		)
		return
	}
	c := metric.NewObserveConfig(opts)
	oImpl.observe(v, c.Attributes())
}

func (r observer) ObserveInt64(o metric.Int64Observable, v int64, opts ...metric.ObserveOption) {
	var oImpl int64Observable
	switch conv := o.(type) {
	case int64Observable:
		oImpl = conv
	case interface {
		Unwrap() metric.Observable
	}:
		// Unwrap any global.
		async := conv.Unwrap()
		var ok bool
		if oImpl, ok = async.(int64Observable); !ok {
			global.Error(errUnknownObserver, "failed to record asynchronous")
			return
		}
	default:
		global.Error(errUnknownObserver, "failed to record")
		return
	}

	if _, registered := r.int64[oImpl.observablID]; !registered {
		global.Error(errUnregObserver, "failed to record",
			"name", oImpl.name,
			"description", oImpl.description,
			"unit", oImpl.unit,
			"number", fmt.Sprintf("%T", int64(0)),
		)
		return
	}
	c := metric.NewObserveConfig(opts)
	oImpl.observe(v, c.Attributes())
}

type noopRegister struct{ embedded.Registration }

func (noopRegister) Unregister() error {
	return nil
}

// int64InstProvider provides int64 OpenTelemetry instruments.
type int64InstProvider struct{ *meter }

func (p int64InstProvider) aggs(kind InstrumentKind, name, desc, u string) ([]aggregate.Measure[int64], error) {
	inst := Instrument{
		Name:        name,
		Description: desc,
		Unit:        u,
		Kind:        kind,
		Scope:       p.scope,
	}
	return p.int64Resolver.Aggregators(inst)
}

func (p int64InstProvider) histogramAggs(name string, cfg metric.Int64HistogramConfig) ([]aggregate.Measure[int64], error) {
	boundaries := cfg.ExplicitBucketBoundaries()
	aggError := AggregationExplicitBucketHistogram{Boundaries: boundaries}.err()
	if aggError != nil {
		// If boundaries are invalid, ignore them.
		boundaries = nil
	}
	inst := Instrument{
		Name:        name,
		Description: cfg.Description(),
		Unit:        cfg.Unit(),
		Kind:        InstrumentKindHistogram,
		Scope:       p.scope,
	}
	measures, err := p.int64Resolver.HistogramAggregators(inst, boundaries)
	return measures, errors.Join(aggError, err)
}

// lookup returns the resolved instrumentImpl.
func (p int64InstProvider) lookup(kind InstrumentKind, name, desc, u string) (*int64Inst, error) {
	aggs, err := p.aggs(kind, name, desc, u)
	return &int64Inst{measures: aggs}, err
}

// lookupHistogram returns the resolved instrumentImpl.
func (p int64InstProvider) lookupHistogram(name string, cfg metric.Int64HistogramConfig) (*int64Inst, error) {
	aggs, err := p.histogramAggs(name, cfg)
	return &int64Inst{measures: aggs}, err
}

// float64InstProvider provides float64 OpenTelemetry instruments.
type float64InstProvider struct{ *meter }

func (p float64InstProvider) aggs(kind InstrumentKind, name, desc, u string) ([]aggregate.Measure[float64], error) {
	inst := Instrument{
		Name:        name,
		Description: desc,
		Unit:        u,
		Kind:        kind,
		Scope:       p.scope,
	}
	return p.float64Resolver.Aggregators(inst)
}

func (p float64InstProvider) histogramAggs(name string, cfg metric.Float64HistogramConfig) ([]aggregate.Measure[float64], error) {
	boundaries := cfg.ExplicitBucketBoundaries()
	aggError := AggregationExplicitBucketHistogram{Boundaries: boundaries}.err()
	if aggError != nil {
		// If boundaries are invalid, ignore them.
		boundaries = nil
	}
	inst := Instrument{
		Name:        name,
		Description: cfg.Description(),
		Unit:        cfg.Unit(),
		Kind:        InstrumentKindHistogram,
		Scope:       p.scope,
	}
	measures, err := p.float64Resolver.HistogramAggregators(inst, boundaries)
	return measures, errors.Join(aggError, err)
}

// lookup returns the resolved instrumentImpl.
func (p float64InstProvider) lookup(kind InstrumentKind, name, desc, u string) (*float64Inst, error) {
	aggs, err := p.aggs(kind, name, desc, u)
	return &float64Inst{measures: aggs}, err
}

// lookupHistogram returns the resolved instrumentImpl.
func (p float64InstProvider) lookupHistogram(name string, cfg metric.Float64HistogramConfig) (*float64Inst, error) {
	aggs, err := p.histogramAggs(name, cfg)
	return &float64Inst{measures: aggs}, err
}

type int64ObservProvider struct{ *meter }

func (p int64ObservProvider) lookup(kind InstrumentKind, name, desc, u string) (int64Observable, error) {
	aggs, err := (int64InstProvider)(p).aggs(kind, name, desc, u)
	return newInt64Observable(p.meter, kind, name, desc, u, aggs), err
}

func (p int64ObservProvider) registerCallbacks(inst int64Observable, cBacks []metric.Int64Callback) {
	if inst.observable == nil || len(inst.measures) == 0 {
		// Drop aggregator.
		return
	}

	for _, cBack := range cBacks {
		p.pipes.registerCallback(p.callback(inst, cBack))
	}
}

func (p int64ObservProvider) callback(i int64Observable, f metric.Int64Callback) func(context.Context) error {
	inst := int64Observer{int64Observable: i}
	return func(ctx context.Context) error { return f(ctx, inst) }
}

type int64Observer struct {
	embedded.Int64Observer
	int64Observable
}

func (o int64Observer) Observe(val int64, opts ...metric.ObserveOption) {
	c := metric.NewObserveConfig(opts)
	o.observe(val, c.Attributes())
}

type float64ObservProvider struct{ *meter }

func (p float64ObservProvider) lookup(kind InstrumentKind, name, desc, u string) (float64Observable, error) {
	aggs, err := (float64InstProvider)(p).aggs(kind, name, desc, u)
	return newFloat64Observable(p.meter, kind, name, desc, u, aggs), err
}

func (p float64ObservProvider) registerCallbacks(inst float64Observable, cBacks []metric.Float64Callback) {
	if inst.observable == nil || len(inst.measures) == 0 {
		// Drop aggregator.
		return
	}

	for _, cBack := range cBacks {
		p.pipes.registerCallback(p.callback(inst, cBack))
	}
}

func (p float64ObservProvider) callback(i float64Observable, f metric.Float64Callback) func(context.Context) error {
	inst := float64Observer{float64Observable: i}
	return func(ctx context.Context) error { return f(ctx, inst) }
}

type float64Observer struct {
	embedded.Float64Observer
	float64Observable
}

func (o float64Observer) Observe(val float64, opts ...metric.ObserveOption) {
	c := metric.NewObserveConfig(opts)
	o.observe(val, c.Attributes())
}
