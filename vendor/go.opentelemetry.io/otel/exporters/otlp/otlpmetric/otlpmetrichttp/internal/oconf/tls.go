// Code created by gotmpl. DO NOT MODIFY.
// source: internal/shared/otlp/otlpmetric/oconf/tls.go.tmpl

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oconf // import "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp/internal/oconf"

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
)

// ReadTLSConfigFromFile reads a PEM certificate file and creates
// a tls.Config that will use this certificate to verify a server certificate.
func ReadTLSConfigFromFile(path string) (*tls.Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return CreateTLSConfig(b)
}

// CreateTLSConfig creates a tls.Config from a raw certificate bytes
// to verify a server certificate.
func CreateTLSConfig(certBytes []byte) (*tls.Config, error) {
	cp := x509.NewCertPool()
	if ok := cp.AppendCertsFromPEM(certBytes); !ok {
		return nil, errors.New("failed to append certificate to the cert pool")
	}

	return &tls.Config{
		RootCAs: cp,
	}, nil
}
