//go:build windows
// +build windows

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package wintls

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"

	"github.com/google/certtostore"
	"golang.org/x/sys/windows"
)

type CertResource = io.Closer

type WindowsCertResource struct {
	store       *certtostore.WinCertStore
	certContext *windows.CertContext
}

// NewWindowsCertResource creates a new WindowsCertResource for managing Windows certificate resources
func NewWindowsCertResource(store *certtostore.WinCertStore, certContext *windows.CertContext) *WindowsCertResource {
	return &WindowsCertResource{
		store:       store,
		certContext: certContext,
	}
}

func (w *WindowsCertResource) Close() error {
	var result error
	if w.certContext != nil {
		if err := certtostore.FreeCertContext(w.certContext); err != nil {
			result = err
		}
		w.certContext = nil
	}
	if w.store != nil {
		if err := w.store.Close(); err != nil {
			result = err
		}
		w.store = nil
	}
	return result
}

// Returns tls.Config, certPool, CertResource for caller-managed cleanup
func SetupTLSFromWindowsCertStore(ctx context.Context, commonName string) (*tls.Config, *x509.CertPool, io.Closer, error) {
	// Open the Windows Certificate Store (My store)
	store, err := certtostore.OpenWinCertStore(certtostore.ProviderMSSoftware, "My", []string{}, nil, false)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to open Windows Certificate Store (My): %w", err)
	}

	leafCert, certContext, certChains, err := store.CertByCommonName(commonName)
	if err != nil {
		store.Close()
		return nil, nil, nil, fmt.Errorf("failed to find certificate with context: %w", err)
	}
	if leafCert == nil {
		store.Close()
		return nil, nil, nil, fmt.Errorf("leaf certificate is nil for common name: %v", commonName)
	}
	if certContext == nil {
		store.Close()
		return nil, nil, nil, fmt.Errorf("certificate context is nil for common name: %v", commonName)
	}

	// Retrieve the private key from certtostore
	key, err := store.CertKey(certContext)
	if err != nil {
		certtostore.FreeCertContext(certContext)
		store.Close()
		return nil, nil, nil, fmt.Errorf("failed to retrieve private key: %w", err)
	}
	if key == nil {
		certtostore.FreeCertContext(certContext)
		store.Close()
		return nil, nil, nil, fmt.Errorf("retrieved private key is nil")
	}

	// Convert the x509 certificate chain to a CertPool and chain bytes
	certPool := x509.NewCertPool()
	var validIntermediates [][]byte
	for _, cert := range certChains {
		for _, c := range cert {
			// Remove leaf certificate from the chain.
			if bytes.Equal(c.Raw, leafCert.Raw) {
				continue // Skip the leaf certificate
			}
			certPool.AddCert(c)
			validIntermediates = append(validIntermediates, c.Raw)
		}
	}
	// Create a TLS certificate using the retrieved certificate and private key
	tlsCert := tls.Certificate{
		PrivateKey:  key,
		Leaf:        leafCert,
		Certificate: append([][]byte{leafCert.Raw}, validIntermediates...),
	}
	// Create TLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}
	// Create the resource manager for cleanup
	certResource := NewWindowsCertResource(store, certContext)
	return tlsConfig, certPool, certResource, nil
}
