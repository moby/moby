// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package detect

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk"
	"go.opentelemetry.io/otel/sdk/resource"
)

var (
	ServiceName string

	detectedResource     *resource.Resource
	detectedResourceOnce sync.Once
)

// schemaURL is the OpenTelemetry semantic conventions schema URL. See [OTel Schema].
//
// [OTel Schema]: https://opentelemetry.io/docs/specs/otel/schemas/
const schemaURL = "https://opentelemetry.io/schemas/1.37.0"

// serviceNameKey is the OpenTelemetry semantic convention key for the
// service name. See [service.name].
//
// [service.name]: https://opentelemetry.io/docs/specs/semconv/registry/attributes/service/#service-name
const serviceNameKey = "service.name"

// telemetrySDKNameKey is the OpenTelemetry semantic convention key for
// the telemetry SDK name. See [telemetry.sdk.name].
//
// [telemetry.sdk.name]: https://opentelemetry.io/docs/specs/semconv/registry/attributes/telemetry/#telemetry-sdk-name
const telemetrySDKNameKey = "telemetry.sdk.name"

// telemetrySDKLanguageKey is the OpenTelemetry semantic convention key for
// the telemetry SDK language. See [telemetry.sdk.language].
//
// [telemetry.sdk.language]: https://opentelemetry.io/docs/specs/semconv/registry/attributes/telemetry/#telemetry-sdk-language
const telemetrySDKLanguageKey = "telemetry.sdk.language"

// telemetrySDKVersionKey is the OpenTelemetry semantic convention key for
// the telemetry SDK version. See [telemetry.sdk.version].
//
// [telemetry.sdk.version]: https://opentelemetry.io/docs/specs/semconv/registry/attributes/telemetry/#telemetry-sdk-version
const telemetrySDKVersionKey = "telemetry.sdk.version"

func Resource() *resource.Resource {
	detectedResourceOnce.Do(func() {
		res, err := resource.New(context.Background(),
			resource.WithDetectors(serviceNameDetector{}),
			resource.WithFromEnv(),
			resource.WithDetectors(telemetrySDK{}),
		)
		if err != nil {
			otel.Handle(err)
		}
		detectedResource = res
	})
	return detectedResource
}

// OverrideResource overrides the resource returned from Resource.
//
// This must be invoked before Resource is called otherwise it is a no-op.
func OverrideResource(res *resource.Resource) {
	detectedResourceOnce.Do(func() {
		detectedResource = res
	})
}

type (
	telemetrySDK        struct{}
	serviceNameDetector struct{}
)

var (
	_ resource.Detector = telemetrySDK{}
	_ resource.Detector = serviceNameDetector{}
)

func (serviceNameDetector) Detect(ctx context.Context) (*resource.Resource, error) {
	return resource.StringDetector(
		schemaURL,
		serviceNameKey,
		func() (string, error) {
			if ServiceName != "" {
				return ServiceName, nil
			}
			return filepath.Base(os.Args[0]), nil
		},
	).Detect(ctx)
}

// Detect returns a [*resource.Resource] that describes the OpenTelemetry SDK used.
func (telemetrySDK) Detect(context.Context) (*resource.Resource, error) {
	return resource.NewWithAttributes(
		schemaURL,
		attribute.String(telemetrySDKNameKey, "opentelemetry"),
		attribute.String(telemetrySDKLanguageKey, "go"),
		attribute.String(telemetrySDKVersionKey, sdk.Version()),
	), nil
}
