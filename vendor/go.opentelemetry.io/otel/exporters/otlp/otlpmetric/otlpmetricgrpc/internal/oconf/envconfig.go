// Code created by gotmpl. DO NOT MODIFY.
// source: internal/shared/otlp/otlpmetric/oconf/envconfig.go.tmpl

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

package oconf // import "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc/internal/oconf"

import (
	"crypto/tls"
	"crypto/x509"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc/internal/envconfig"
	"go.opentelemetry.io/otel/internal/global"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// DefaultEnvOptionsReader is the default environments reader.
var DefaultEnvOptionsReader = envconfig.EnvOptionsReader{
	GetEnv:    os.Getenv,
	ReadFile:  os.ReadFile,
	Namespace: "OTEL_EXPORTER_OTLP",
}

// ApplyGRPCEnvConfigs applies the env configurations for gRPC.
func ApplyGRPCEnvConfigs(cfg Config) Config {
	opts := getOptionsFromEnv()
	for _, opt := range opts {
		cfg = opt.ApplyGRPCOption(cfg)
	}
	return cfg
}

// ApplyHTTPEnvConfigs applies the env configurations for HTTP.
func ApplyHTTPEnvConfigs(cfg Config) Config {
	opts := getOptionsFromEnv()
	for _, opt := range opts {
		cfg = opt.ApplyHTTPOption(cfg)
	}
	return cfg
}

func getOptionsFromEnv() []GenericOption {
	opts := []GenericOption{}

	tlsConf := &tls.Config{}
	DefaultEnvOptionsReader.Apply(
		envconfig.WithURL("ENDPOINT", func(u *url.URL) {
			opts = append(opts, withEndpointScheme(u))
			opts = append(opts, newSplitOption(func(cfg Config) Config {
				cfg.Metrics.Endpoint = u.Host
				// For OTLP/HTTP endpoint URLs without a per-signal
				// configuration, the passed endpoint is used as a base URL
				// and the signals are sent to these paths relative to that.
				cfg.Metrics.URLPath = path.Join(u.Path, DefaultMetricsPath)
				return cfg
			}, withEndpointForGRPC(u)))
		}),
		envconfig.WithURL("METRICS_ENDPOINT", func(u *url.URL) {
			opts = append(opts, withEndpointScheme(u))
			opts = append(opts, newSplitOption(func(cfg Config) Config {
				cfg.Metrics.Endpoint = u.Host
				// For endpoint URLs for OTLP/HTTP per-signal variables, the
				// URL MUST be used as-is without any modification. The only
				// exception is that if an URL contains no path part, the root
				// path / MUST be used.
				path := u.Path
				if path == "" {
					path = "/"
				}
				cfg.Metrics.URLPath = path
				return cfg
			}, withEndpointForGRPC(u)))
		}),
		envconfig.WithCertPool("CERTIFICATE", func(p *x509.CertPool) { tlsConf.RootCAs = p }),
		envconfig.WithCertPool("METRICS_CERTIFICATE", func(p *x509.CertPool) { tlsConf.RootCAs = p }),
		envconfig.WithClientCert("CLIENT_CERTIFICATE", "CLIENT_KEY", func(c tls.Certificate) { tlsConf.Certificates = []tls.Certificate{c} }),
		envconfig.WithClientCert("METRICS_CLIENT_CERTIFICATE", "METRICS_CLIENT_KEY", func(c tls.Certificate) { tlsConf.Certificates = []tls.Certificate{c} }),
		envconfig.WithBool("INSECURE", func(b bool) { opts = append(opts, withInsecure(b)) }),
		envconfig.WithBool("METRICS_INSECURE", func(b bool) { opts = append(opts, withInsecure(b)) }),
		withTLSConfig(tlsConf, func(c *tls.Config) { opts = append(opts, WithTLSClientConfig(c)) }),
		envconfig.WithHeaders("HEADERS", func(h map[string]string) { opts = append(opts, WithHeaders(h)) }),
		envconfig.WithHeaders("METRICS_HEADERS", func(h map[string]string) { opts = append(opts, WithHeaders(h)) }),
		WithEnvCompression("COMPRESSION", func(c Compression) { opts = append(opts, WithCompression(c)) }),
		WithEnvCompression("METRICS_COMPRESSION", func(c Compression) { opts = append(opts, WithCompression(c)) }),
		envconfig.WithDuration("TIMEOUT", func(d time.Duration) { opts = append(opts, WithTimeout(d)) }),
		envconfig.WithDuration("METRICS_TIMEOUT", func(d time.Duration) { opts = append(opts, WithTimeout(d)) }),
		withEnvTemporalityPreference("METRICS_TEMPORALITY_PREFERENCE", func(t metric.TemporalitySelector) { opts = append(opts, WithTemporalitySelector(t)) }),
		withEnvAggPreference("METRICS_DEFAULT_HISTOGRAM_AGGREGATION", func(a metric.AggregationSelector) { opts = append(opts, WithAggregationSelector(a)) }),
	)

	return opts
}

func withEndpointForGRPC(u *url.URL) func(cfg Config) Config {
	return func(cfg Config) Config {
		// For OTLP/gRPC endpoints, this is the target to which the
		// exporter is going to send telemetry.
		cfg.Metrics.Endpoint = path.Join(u.Host, u.Path)
		return cfg
	}
}

// WithEnvCompression retrieves the specified config and passes it to ConfigFn as a Compression.
func WithEnvCompression(n string, fn func(Compression)) func(e *envconfig.EnvOptionsReader) {
	return func(e *envconfig.EnvOptionsReader) {
		if v, ok := e.GetEnvValue(n); ok {
			cp := NoCompression
			if v == "gzip" {
				cp = GzipCompression
			}

			fn(cp)
		}
	}
}

func withEndpointScheme(u *url.URL) GenericOption {
	switch strings.ToLower(u.Scheme) {
	case "http", "unix":
		return WithInsecure()
	default:
		return WithSecure()
	}
}

// revive:disable-next-line:flag-parameter
func withInsecure(b bool) GenericOption {
	if b {
		return WithInsecure()
	}
	return WithSecure()
}

func withTLSConfig(c *tls.Config, fn func(*tls.Config)) func(e *envconfig.EnvOptionsReader) {
	return func(e *envconfig.EnvOptionsReader) {
		if c.RootCAs != nil || len(c.Certificates) > 0 {
			fn(c)
		}
	}
}

func withEnvTemporalityPreference(n string, fn func(metric.TemporalitySelector)) func(e *envconfig.EnvOptionsReader) {
	return func(e *envconfig.EnvOptionsReader) {
		if s, ok := e.GetEnvValue(n); ok {
			switch strings.ToLower(s) {
			case "cumulative":
				fn(cumulativeTemporality)
			case "delta":
				fn(deltaTemporality)
			case "lowmemory":
				fn(lowMemory)
			default:
				global.Warn("OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE is set to an invalid value, ignoring.", "value", s)
			}
		}
	}
}

func cumulativeTemporality(metric.InstrumentKind) metricdata.Temporality {
	return metricdata.CumulativeTemporality
}

func deltaTemporality(ik metric.InstrumentKind) metricdata.Temporality {
	switch ik {
	case metric.InstrumentKindCounter, metric.InstrumentKindHistogram, metric.InstrumentKindObservableCounter:
		return metricdata.DeltaTemporality
	default:
		return metricdata.CumulativeTemporality
	}
}

func lowMemory(ik metric.InstrumentKind) metricdata.Temporality {
	switch ik {
	case metric.InstrumentKindCounter, metric.InstrumentKindHistogram:
		return metricdata.DeltaTemporality
	default:
		return metricdata.CumulativeTemporality
	}
}

func withEnvAggPreference(n string, fn func(metric.AggregationSelector)) func(e *envconfig.EnvOptionsReader) {
	return func(e *envconfig.EnvOptionsReader) {
		if s, ok := e.GetEnvValue(n); ok {
			switch strings.ToLower(s) {
			case "explicit_bucket_histogram":
				fn(metric.DefaultAggregationSelector)
			case "base2_exponential_bucket_histogram":
				fn(func(kind metric.InstrumentKind) metric.Aggregation {
					if kind == metric.InstrumentKindHistogram {
						return metric.AggregationBase2ExponentialHistogram{
							MaxSize:  160,
							MaxScale: 20,
							NoMinMax: false,
						}
					}
					return metric.DefaultAggregationSelector(kind)
				})
			default:
				global.Warn("OTEL_EXPORTER_OTLP_METRICS_DEFAULT_HISTOGRAM_AGGREGATION is set to an invalid value, ignoring.", "value", s)
			}
		}
	}
}
