// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metric // import "go.opentelemetry.io/otel/sdk/metric"

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
)

// config contains configuration options for a MeterProvider.
type config struct {
	res     *resource.Resource
	readers []Reader
	views   []View
}

// readerSignals returns a force-flush and shutdown function for a
// MeterProvider to call in their respective options. All Readers c contains
// will have their force-flush and shutdown methods unified into returned
// single functions.
func (c config) readerSignals() (forceFlush, shutdown func(context.Context) error) {
	var fFuncs, sFuncs []func(context.Context) error
	for _, r := range c.readers {
		sFuncs = append(sFuncs, r.Shutdown)
		if f, ok := r.(interface{ ForceFlush(context.Context) error }); ok {
			fFuncs = append(fFuncs, f.ForceFlush)
		}
	}

	return unify(fFuncs), unifyShutdown(sFuncs)
}

// unify unifies calling all of funcs into a single function call. All errors
// returned from calls to funcs will be unify into a single error return
// value.
func unify(funcs []func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		var errs []error
		for _, f := range funcs {
			if err := f(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		return unifyErrors(errs)
	}
}

// unifyErrors combines multiple errors into a single error.
func unifyErrors(errs []error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return fmt.Errorf("%v", errs)
	}
}

// unifyShutdown unifies calling all of funcs once for a shutdown. If called
// more than once, an ErrReaderShutdown error is returned.
func unifyShutdown(funcs []func(context.Context) error) func(context.Context) error {
	f := unify(funcs)
	var once sync.Once
	return func(ctx context.Context) error {
		err := ErrReaderShutdown
		once.Do(func() { err = f(ctx) })
		return err
	}
}

// newConfig returns a config configured with options.
func newConfig(options []Option) config {
	conf := config{res: resource.Default()}
	for _, o := range options {
		conf = o.apply(conf)
	}
	return conf
}

// Option applies a configuration option value to a MeterProvider.
type Option interface {
	apply(config) config
}

// optionFunc applies a set of options to a config.
type optionFunc func(config) config

// apply returns a config with option(s) applied.
func (o optionFunc) apply(conf config) config {
	return o(conf)
}

// WithResource associates a Resource with a MeterProvider. This Resource
// represents the entity producing telemetry and is associated with all Meters
// the MeterProvider will create.
//
// By default, if this Option is not used, the default Resource from the
// go.opentelemetry.io/otel/sdk/resource package will be used.
func WithResource(res *resource.Resource) Option {
	return optionFunc(func(conf config) config {
		var err error
		conf.res, err = resource.Merge(resource.Environment(), res)
		if err != nil {
			otel.Handle(err)
		}
		return conf
	})
}

// WithReader associates Reader r with a MeterProvider.
//
// By default, if this option is not used, the MeterProvider will perform no
// operations; no data will be exported without a Reader.
func WithReader(r Reader) Option {
	return optionFunc(func(cfg config) config {
		if r == nil {
			return cfg
		}
		cfg.readers = append(cfg.readers, r)
		return cfg
	})
}

// WithView associates views with a MeterProvider.
//
// Views are appended to existing ones in a MeterProvider if this option is
// used multiple times.
//
// By default, if this option is not used, the MeterProvider will use the
// default view.
func WithView(views ...View) Option {
	return optionFunc(func(cfg config) config {
		cfg.views = append(cfg.views, views...)
		return cfg
	})
}
