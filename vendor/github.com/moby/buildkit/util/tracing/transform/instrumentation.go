package transform

import (
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"

	"go.opentelemetry.io/otel/sdk/instrumentation"
)

func instrumentationScope(is *commonpb.InstrumentationScope) instrumentation.Scope {
	if is == nil {
		return instrumentation.Scope{}
	}
	return instrumentation.Scope{
		Name:    is.Name,
		Version: is.Version,
	}
}
