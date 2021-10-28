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

package resource // import "go.opentelemetry.io/otel/sdk/resource"

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
)

// config contains configuration for Resource creation.
type config struct {
	// detectors that will be evaluated.
	detectors []Detector
	// SchemaURL to associate with the Resource.
	schemaURL string
}

// Option is the interface that applies a configuration option.
type Option interface {
	// apply sets the Option value of a config.
	apply(*config)
}

// WithAttributes adds attributes to the configured Resource.
func WithAttributes(attributes ...attribute.KeyValue) Option {
	return WithDetectors(detectAttributes{attributes})
}

type detectAttributes struct {
	attributes []attribute.KeyValue
}

func (d detectAttributes) Detect(context.Context) (*Resource, error) {
	return NewSchemaless(d.attributes...), nil
}

// WithDetectors adds detectors to be evaluated for the configured resource.
func WithDetectors(detectors ...Detector) Option {
	return detectorsOption{detectors: detectors}
}

type detectorsOption struct {
	detectors []Detector
}

func (o detectorsOption) apply(cfg *config) {
	cfg.detectors = append(cfg.detectors, o.detectors...)
}

// WithBuiltinDetectors adds the built detectors to the configured resource.
func WithBuiltinDetectors() Option {
	return WithDetectors(telemetrySDK{},
		host{},
		fromEnv{})
}

// WithFromEnv adds attributes from environment variables to the configured resource.
func WithFromEnv() Option {
	return WithDetectors(fromEnv{})
}

// WithHost adds attributes from the host to the configured resource.
func WithHost() Option {
	return WithDetectors(host{})
}

// WithTelemetrySDK adds TelemetrySDK version info to the configured resource.
func WithTelemetrySDK() Option {
	return WithDetectors(telemetrySDK{})
}

// WithSchemaURL sets the schema URL for the configured resource.
func WithSchemaURL(schemaURL string) Option {
	return schemaURLOption(schemaURL)
}

type schemaURLOption string

func (o schemaURLOption) apply(cfg *config) {
	cfg.schemaURL = string(o)
}
