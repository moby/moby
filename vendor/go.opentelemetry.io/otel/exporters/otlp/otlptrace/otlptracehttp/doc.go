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

/*
Package otlptracehttp provides an OTLP span exporter using HTTP with protobuf payloads.
By default the telemetry is sent to https://localhost:4318/v1/traces.

Exporter should be created using [New].

The environment variables described below can be used for configuration.

OTEL_EXPORTER_OTLP_ENDPOINT (default: "https://localhost:4318") -
target base URL ("/v1/traces" is appended) to which the exporter sends telemetry.
The value must contain a scheme ("http" or "https") and host.
The value may additionally contain a port and a path.
The value should not contain a query string or fragment.
The configuration can be overridden by OTEL_EXPORTER_OTLP_TRACES_ENDPOINT
environment variable and by [WithEndpoint], [WithInsecure] options.

OTEL_EXPORTER_OTLP_TRACES_ENDPOINT (default: "https://localhost:4318/v1/traces") -
target URL to which the exporter sends telemetry.
The value must contain a scheme ("http" or "https") and host.
The value may additionally contain a port and a path.
The value should not contain a query string or fragment.
The configuration can be overridden by [WithEndpoint], [WitnInsecure], [WithURLPath] options.

OTEL_EXPORTER_OTLP_HEADERS, OTEL_EXPORTER_OTLP_TRACES_HEADERS (default: none) -
key-value pairs used as headers associated with HTTP requests.
The value is expected to be represented in a format matching to the [W3C Baggage HTTP Header Content Format],
except that additional semi-colon delimited metadata is not supported.
Example value: "key1=value1,key2=value2".
OTEL_EXPORTER_OTLP_TRACES_HEADERS takes precedence over OTEL_EXPORTER_OTLP_HEADERS.
The configuration can be overridden by [WithHeaders] option.

OTEL_EXPORTER_OTLP_TIMEOUT, OTEL_EXPORTER_OTLP_TRACES_TIMEOUT (default: "10000") -
maximum time in milliseconds the OTLP exporter waits for each batch export.
OTEL_EXPORTER_OTLP_TRACES_TIMEOUT takes precedence over OTEL_EXPORTER_OTLP_TIMEOUT.
The configuration can be overridden by [WithTimeout] option.

OTEL_EXPORTER_OTLP_COMPRESSION, OTEL_EXPORTER_OTLP_TRACES_COMPRESSION (default: none) -
the compression strategy the exporter uses to compress the HTTP body.
Supported value: "gzip".
OTEL_EXPORTER_OTLP_TRACES_COMPRESSION takes precedence over OTEL_EXPORTER_OTLP_COMPRESSION.
The configuration can be overridden by [WithCompression] option.

OTEL_EXPORTER_OTLP_CERTIFICATE, OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE (default: none) -
the filepath to the trusted certificate to use when verifying a server's TLS credentials.
OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE takes precedence over OTEL_EXPORTER_OTLP_CERTIFICATE.
The configuration can be overridden by [WithTLSClientConfig] option.

OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE, OTEL_EXPORTER_OTLP_TRACES_CLIENT_CERTIFICATE (default: none) -
the filepath to the client certificate/chain trust for clients private key to use in mTLS communication in PEM format.
OTEL_EXPORTER_OTLP_TRACES_CLIENT_CERTIFICATE takes precedence over OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE.
The configuration can be overridden by [WithTLSClientConfig] option.

OTEL_EXPORTER_OTLP_CLIENT_KEY, OTEL_EXPORTER_OTLP_TRACES_CLIENT_KEY (default: none) -
the filepath  to the clients private key to use in mTLS communication in PEM format.
OTEL_EXPORTER_OTLP_TRACES_CLIENT_KEY takes precedence over OTEL_EXPORTER_OTLP_CLIENT_KEY.
The configuration can be overridden by [WithTLSClientConfig] option.

[W3C Baggage HTTP Header Content Format]: https://www.w3.org/TR/baggage/#header-content
*/
package otlptracehttp // import "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
