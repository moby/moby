// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package log // import "go.opentelemetry.io/otel/sdk/log"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk"
	"go.opentelemetry.io/otel/sdk/log/internal/x"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.opentelemetry.io/otel/semconv/v1.39.0/otelconv"
)

// newRecordCounterIncr returns a function that increments the log record
// counter metric. If observability is disabled, it returns nil.
func newRecordCounterIncr() (func(context.Context), error) {
	if !x.Observability.Enabled() {
		return nil, nil
	}

	m := otel.GetMeterProvider().Meter(
		"go.opentelemetry.io/otel/sdk/log",
		metric.WithInstrumentationVersion(sdk.Version()),
		metric.WithSchemaURL(semconv.SchemaURL),
	)

	created, err := otelconv.NewSDKLogCreated(m)
	if err != nil {
		err = fmt.Errorf("failed to create log created metric: %w", err)
		return nil, err
	}
	inst := created.Inst()
	f := func(ctx context.Context) { inst.Add(ctx, 1) }
	return f, nil
}
