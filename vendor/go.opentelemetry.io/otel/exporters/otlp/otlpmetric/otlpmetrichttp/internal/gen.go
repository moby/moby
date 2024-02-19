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

package internal // import "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp/internal"

//go:generate gotmpl --body=../../../../../internal/shared/otlp/partialsuccess.go.tmpl "--data={}" --out=partialsuccess.go
//go:generate gotmpl --body=../../../../../internal/shared/otlp/partialsuccess_test.go.tmpl "--data={}" --out=partialsuccess_test.go

//go:generate gotmpl --body=../../../../../internal/shared/otlp/retry/retry.go.tmpl "--data={}" --out=retry/retry.go
//go:generate gotmpl --body=../../../../../internal/shared/otlp/retry/retry_test.go.tmpl "--data={}" --out=retry/retry_test.go

//go:generate gotmpl --body=../../../../../internal/shared/otlp/envconfig/envconfig.go.tmpl "--data={}" --out=envconfig/envconfig.go
//go:generate gotmpl --body=../../../../../internal/shared/otlp/envconfig/envconfig_test.go.tmpl "--data={}" --out=envconfig/envconfig_test.go

//go:generate gotmpl --body=../../../../../internal/shared/otlp/otlpmetric/oconf/envconfig.go.tmpl "--data={\"envconfigImportPath\": \"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp/internal/envconfig\"}" --out=oconf/envconfig.go
//go:generate gotmpl --body=../../../../../internal/shared/otlp/otlpmetric/oconf/envconfig_test.go.tmpl "--data={}" --out=oconf/envconfig_test.go
//go:generate gotmpl --body=../../../../../internal/shared/otlp/otlpmetric/oconf/options.go.tmpl "--data={\"retryImportPath\": \"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp/internal/retry\"}" --out=oconf/options.go
//go:generate gotmpl --body=../../../../../internal/shared/otlp/otlpmetric/oconf/options_test.go.tmpl "--data={\"envconfigImportPath\": \"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp/internal/envconfig\"}" --out=oconf/options_test.go
//go:generate gotmpl --body=../../../../../internal/shared/otlp/otlpmetric/oconf/optiontypes.go.tmpl "--data={}" --out=oconf/optiontypes.go
//go:generate gotmpl --body=../../../../../internal/shared/otlp/otlpmetric/oconf/tls.go.tmpl "--data={}" --out=oconf/tls.go

//go:generate gotmpl --body=../../../../../internal/shared/otlp/otlpmetric/otest/client.go.tmpl "--data={}" --out=otest/client.go
//go:generate gotmpl --body=../../../../../internal/shared/otlp/otlpmetric/otest/client_test.go.tmpl "--data={\"internalImportPath\": \"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp/internal\"}" --out=otest/client_test.go
//go:generate gotmpl --body=../../../../../internal/shared/otlp/otlpmetric/otest/collector.go.tmpl "--data={\"oconfImportPath\": \"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp/internal/oconf\"}" --out=otest/collector.go

//go:generate gotmpl --body=../../../../../internal/shared/otlp/otlpmetric/transform/attribute.go.tmpl "--data={}" --out=transform/attribute.go
//go:generate gotmpl --body=../../../../../internal/shared/otlp/otlpmetric/transform/attribute_test.go.tmpl "--data={}" --out=transform/attribute_test.go
//go:generate gotmpl --body=../../../../../internal/shared/otlp/otlpmetric/transform/error.go.tmpl "--data={}" --out=transform/error.go
//go:generate gotmpl --body=../../../../../internal/shared/otlp/otlpmetric/transform/error_test.go.tmpl "--data={}" --out=transform/error_test.go
//go:generate gotmpl --body=../../../../../internal/shared/otlp/otlpmetric/transform/metricdata.go.tmpl "--data={}" --out=transform/metricdata.go
//go:generate gotmpl --body=../../../../../internal/shared/otlp/otlpmetric/transform/metricdata_test.go.tmpl "--data={}" --out=transform/metricdata_test.go
