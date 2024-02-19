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

// Package otlpmetric provides an OpenTelemetry metric Exporter that can be
// used with PeriodicReader. It transforms metricdata into OTLP and transmits
// the transformed data to OTLP receivers. The Exporter is configurable to use
// different Clients, each using a distinct transport protocol to communicate
// to an OTLP receiving endpoint.
package otlpmetric // import "go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
