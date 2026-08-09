// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/embedded"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

var now = time.Now

const (
	exceptionTypeKey    = semconv.ExceptionTypeKey
	exceptionMessageKey = semconv.ExceptionMessageKey
)

// Compile-time check logger implements log.Logger.
var _ log.Logger = (*logger)(nil)

type logger struct {
	embedded.Logger

	provider             *LoggerProvider
	instrumentationScope instrumentation.Scope

	// recCntIncr increments the count of log records created. It will be nil
	// if observability is disabled.
	recCntIncr func(context.Context)
}

func newLogger(p *LoggerProvider, scope instrumentation.Scope) *logger {
	l := &logger{
		provider:             p,
		instrumentationScope: scope,
	}

	var err error
	l.recCntIncr, err = newRecordCounterIncr()
	if err != nil {
		otel.Handle(err)
	}
	return l
}

func (l *logger) Emit(ctx context.Context, r log.Record) {
	processors := l.provider.processors
	if len(processors) == 0 {
		if l.provider.stopped.Load() {
			return
		}
		// Emit remains observable without processors, but no lifecycle
		// admission or SDK record construction is needed.
		l.recordCreated(ctx)
		return
	}

	for _, err := range l.emit(ctx, r, processors) {
		otel.Handle(err)
	}
}

func (l *logger) emit(ctx context.Context, r log.Record, processors []Processor) []error {
	if !l.provider.beginProcessorOperation() {
		return nil
	}
	defer l.provider.endProcessorOperation()

	l.recordCreated(ctx)
	newRecord := l.newRecord(ctx, r)
	var errs []error
	for _, processor := range processors {
		if err := processor.OnEmit(ctx, &newRecord); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (l *logger) recordCreated(ctx context.Context) {
	if l.recCntIncr != nil {
		l.recCntIncr(ctx)
	}
}

// Enabled returns true if at least one Processor held by the LoggerProvider
// that created the logger will process for the provided context and param.
//
// Enabled returns false after the LoggerProvider that created l starts shutdown.
//
// If it is not possible to definitively determine the record will be
// processed, true will be returned by default. A value of false will only be
// returned if it can be positively verified that no Processor will process.
func (l *logger) Enabled(ctx context.Context, param log.EnabledParameters) bool {
	p := EnabledParameters{
		InstrumentationScope: l.instrumentationScope,
		Severity:             param.Severity,
		EventName:            param.EventName,
	}

	processors := l.provider.processors
	if len(processors) == 0 || !l.provider.beginProcessorOperation() {
		return false
	}
	defer l.provider.endProcessorOperation()

	for _, processor := range processors {
		if processor.Enabled(ctx, p) {
			// At least one Processor will process the Record.
			return true
		}
	}
	// No Processor will process the record.
	return false
}

func (l *logger) newRecord(ctx context.Context, r log.Record) Record {
	sc := trace.SpanContextFromContext(ctx)

	newRecord := Record{
		eventName:         r.EventName(),
		timestamp:         r.Timestamp(),
		observedTimestamp: r.ObservedTimestamp(),
		severity:          r.Severity(),
		severityText:      r.SeverityText(),

		traceID:    sc.TraceID(),
		spanID:     sc.SpanID(),
		traceFlags: sc.TraceFlags(),

		resource:                  l.provider.resource,
		scope:                     &l.instrumentationScope,
		attributeValueLengthLimit: l.provider.attributeValueLengthLimit,
		attributeCountLimit:       l.provider.attributeCountLimit,
		allowDupKeys:              l.provider.allowDupKeys,
	}
	// This ensures we deduplicate key-value collections in the log body
	newRecord.SetBody(r.Body())

	// This field SHOULD be set once the event is observed by OpenTelemetry.
	if newRecord.observedTimestamp.IsZero() {
		newRecord.observedTimestamp = now()
	}

	// User-provided exception attributes MUST take precedence. Track message
	// and type independently so a supplied value suppresses only its own
	// derivation.
	var hasExceptionMessage, hasExceptionType bool
	r.WalkAttributes(func(kv attribute.KeyValue) bool {
		switch kv.Key {
		case exceptionMessageKey:
			hasExceptionMessage = true
		case exceptionTypeKey:
			hasExceptionType = true
		}
		newRecord.AddAttributes(kv)
		return true
	})

	// Avoid inspecting the error for attributes when the caller has
	// already supplied the attributes.
	if err := r.Err(); err != nil && (!hasExceptionMessage || !hasExceptionType) {
		// Derive missing exception attributes by default, as required by the
		// Logs SDK specification. Attribute limits may constrain generation,
		// so stop once there is no capacity for another attribute.
		var attrs [2]attribute.KeyValue
		n := 0

		// Derived attributes are buffered until flush, so the current attribute
		// count stays unchanged while missing values are prepared.
		hasLimit := newRecord.hasAttributeCountLimit()
		var remaining int
		if hasLimit {
			remaining = newRecord.attributeCountLimit - newRecord.AttributesLen()
		}
		if !hasExceptionMessage {
			if msg := err.Error(); msg != "" {
				if hasLimit && remaining <= n {
					goto flush
				}
				attrs[n] = exceptionMessageKey.String(msg)
				n++
			}
		}
		if !hasExceptionType {
			if errType := errorType(err); errType != "" {
				if hasLimit && remaining <= n {
					goto flush
				}
				attrs[n] = exceptionTypeKey.String(errType)
				n++
			}
		}

	flush:
		if n > 0 {
			newRecord.addAttrs(attrs[:n])
		}
	}

	return newRecord
}

func errorType(err error) string {
	if et, ok := err.(interface{ ErrorType() string }); ok {
		if s := et.ErrorType(); s != "" {
			return s
		}
	}

	t := reflect.TypeOf(unwrapFmtWrapped(err))
	if t == nil {
		return ""
	}

	pkg, name := t.PkgPath(), t.Name()
	if pkg != "" && name != "" {
		return pkg + "." + name
	}

	// The type has no package path or name (predeclared, not-defined,
	// or alias for a not-defined type).
	//
	// The type has no package path or name (predeclared, not-defined,
	// or alias for a not-defined type).
	//
	// This is not guaranteed to be unique, but is a best effort.
	return t.String()
}

var fmtWrapErrorType = reflect.TypeOf(fmt.Errorf("wrapped: %w", errors.New("err")))

func unwrapFmtWrapped(err error) error {
	for reflect.TypeOf(err) == fmtWrapErrorType {
		u := errors.Unwrap(err)
		if u == nil {
			return err // When the wrapped error is nil, use the concrete type of the wrapper.
		}
		err = u
	}
	return err
}
