module go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace

go 1.15

replace go.opentelemetry.io/contrib => ../../../../..

require (
	github.com/google/go-cmp v0.5.6
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/contrib v0.21.0
	go.opentelemetry.io/otel v1.0.0-RC1
	go.opentelemetry.io/otel/oteltest v1.0.0-RC1
	go.opentelemetry.io/otel/trace v1.0.0-RC1
)
