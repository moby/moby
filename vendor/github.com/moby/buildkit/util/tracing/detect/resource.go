package detect

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

var (
	ServiceName string

	detectedResource     *resource.Resource
	detectedResourceOnce sync.Once
)

func Resource() *resource.Resource {
	detectedResourceOnce.Do(func() {
		res, err := resource.New(context.Background(),
			resource.WithDetectors(serviceNameDetector{}),
			resource.WithFromEnv(),
			resource.WithTelemetrySDK(),
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

type serviceNameDetector struct{}

func (serviceNameDetector) Detect(ctx context.Context) (*resource.Resource, error) {
	return resource.StringDetector(
		semconv.SchemaURL,
		semconv.ServiceNameKey,
		func() (string, error) {
			if ServiceName != "" {
				return ServiceName, nil
			}
			return filepath.Base(os.Args[0]), nil
		},
	).Detect(ctx)
}
