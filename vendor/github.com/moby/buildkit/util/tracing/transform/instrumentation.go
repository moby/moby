package transform

import (
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"

	"go.opentelemetry.io/otel/sdk/instrumentation"
)

func instrumentationLibrary(il *commonpb.InstrumentationLibrary) instrumentation.Library {
	if il == nil {
		return instrumentation.Library{}
	}
	return instrumentation.Library{
		Name:    il.Name,
		Version: il.Version,
	}
}
