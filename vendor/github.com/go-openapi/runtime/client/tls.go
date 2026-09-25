// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
)

// TLSClientOptions to configure client authentication with mutual TLS.
type TLSClientOptions struct {
	// Certificate is the path to a PEM-encoded certificate to be used for
	// client authentication. If set then Key must also be set.
	Certificate string

	// LoadedCertificate is the certificate to be used for client authentication.
	// This field is ignored if Certificate is set. If this field is set, LoadedKey
	// is also required.
	LoadedCertificate *x509.Certificate

	// Key is the path to an unencrypted PEM-encoded private key for client
	// authentication. This field is required if Certificate is set.
	Key string

	// LoadedKey is the key for client authentication. This field is required if
	// LoadedCertificate is set.
	LoadedKey crypto.PrivateKey

	// CA is a path to a PEM-encoded certificate that specifies the root certificate
	// to use when validating the TLS certificate presented by the server. If this field
	// (and LoadedCA) is not set, the system certificate pool is used. This field is ignored if LoadedCA
	// is set.
	CA string

	// LoadedCA specifies the root certificate to use when validating the server's TLS certificate.
	// If this field (and CA) is not set, the system certificate pool is used.
	LoadedCA *x509.Certificate

	// LoadedCAPool specifies a pool of RootCAs to use when validating the server's TLS certificate.
	// If set, it will be combined with the other loaded certificates (see LoadedCA and CA).
	// If neither LoadedCA or CA is set, the provided pool will override the system
	// certificate pool.
	//
	// The caller must not use the supplied pool after calling TLSClientAuth.
	LoadedCAPool *x509.CertPool

	// ServerName specifies the hostname to use when verifying the server certificate.
	// If this field is set then InsecureSkipVerify will be ignored and treated as
	// false.
	ServerName string

	// InsecureSkipVerify controls whether the certificate chain and hostname presented
	// by the server are validated. If true, any certificate is accepted.
	InsecureSkipVerify bool

	// VerifyPeerCertificate, if not nil, is called after normal
	// certificate verification. It receives the raw ASN.1 certificates
	// provided by the peer and also any verified chains that normal processing found.
	// If it returns a non-nil error, the handshake is aborted and that error results.
	//
	// If normal verification fails then the handshake will abort before
	// considering this callback. If normal verification is disabled by
	// setting InsecureSkipVerify then this callback will be considered but
	// the verifiedChains argument will always be nil.
	VerifyPeerCertificate func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error

	// VerifyConnection, if not nil, is called after normal certificate
	// verification and after [TLSClientOptions.VerifyPeerCertificate] by either a TLS client or
	// server. It receives the [tls.ConnectionState] which may be inspected.
	//
	// Unlike VerifyPeerCertificate, this callback is invoked on every
	// connection, including resumed ones, making it suitable for checks
	// that must always apply (e.g. certificate pinning).
	//
	// If it returns a non-nil error, the handshake is aborted and that error results.
	VerifyConnection func(tls.ConnectionState) error

	// SessionTicketsDisabled may be set to true to disable session ticket and
	// PSK (resumption) support. Note that on clients, session ticket support is
	// also disabled if ClientSessionCache is nil.
	SessionTicketsDisabled bool

	// ClientSessionCache is a cache of ClientSessionState entries for TLS
	// session resumption. It is only used by clients.
	ClientSessionCache tls.ClientSessionCache

	// Prevents callers using unkeyed fields.
	_ struct{}
}

// TLSClientAuth creates a [tls.Config] for mutual auth.
func TLSClientAuth(opts TLSClientOptions) (*tls.Config, error) {
	// create client tls config
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// load client cert if specified
	if opts.Certificate != "" {
		cert, err := tls.LoadX509KeyPair(opts.Certificate, opts.Key)
		if err != nil {
			return nil, fmt.Errorf("tls client cert: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	} else if opts.LoadedCertificate != nil {
		block := pem.Block{Type: "CERTIFICATE", Bytes: opts.LoadedCertificate.Raw}
		certPem := pem.EncodeToMemory(&block)

		// PKCS#8 covers RSA, ECDSA, Ed25519, X25519 (the key types tls.X509KeyPair
		// understands) and pairs with the canonical "PRIVATE KEY" PEM label.
		keyBytes, err := x509.MarshalPKCS8PrivateKey(opts.LoadedKey)
		if err != nil {
			return nil, fmt.Errorf("tls client priv key: %w", err)
		}

		block = pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}
		keyPem := pem.EncodeToMemory(&block)

		cert, err := tls.X509KeyPair(certPem, keyPem)
		if err != nil {
			return nil, fmt.Errorf("tls client cert: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}

	cfg.InsecureSkipVerify = opts.InsecureSkipVerify

	cfg.VerifyPeerCertificate = opts.VerifyPeerCertificate
	cfg.VerifyConnection = opts.VerifyConnection
	cfg.SessionTicketsDisabled = opts.SessionTicketsDisabled
	cfg.ClientSessionCache = opts.ClientSessionCache

	// When no CA certificate is provided, default to the system cert pool
	// that way when a request is made to a server known by the system trust store,
	// the name is still verified
	switch {
	case opts.LoadedCA != nil:
		caCertPool := basePool(opts.LoadedCAPool)
		caCertPool.AddCert(opts.LoadedCA)
		cfg.RootCAs = caCertPool
	case opts.CA != "":
		// load ca cert
		caCert, err := os.ReadFile(opts.CA)
		if err != nil {
			return nil, fmt.Errorf("tls client ca: %w", err)
		}
		caCertPool := basePool(opts.LoadedCAPool)
		caCertPool.AppendCertsFromPEM(caCert)
		cfg.RootCAs = caCertPool
	case opts.LoadedCAPool != nil:
		cfg.RootCAs = opts.LoadedCAPool
	}

	// apply servername override
	if opts.ServerName != "" {
		cfg.InsecureSkipVerify = false
		cfg.ServerName = opts.ServerName
	}

	return cfg, nil
}

// TLSTransport creates a [http.RoundTripper] for a client transport,suitable for mutual TLS auth.
func TLSTransport(opts TLSClientOptions) (http.RoundTripper, error) {
	cfg, err := TLSClientAuth(opts)
	if err != nil {
		return nil, err
	}

	return &http.Transport{TLSClientConfig: cfg}, nil
}

// TLSClient creates a [http.Client] for mutual auth.
func TLSClient(opts TLSClientOptions) (*http.Client, error) {
	transport, err := TLSTransport(opts)
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: transport}, nil
}

// basePool returns pool if non-nil; otherwise it returns a new empty cert pool.
//
// Clones the pool provided up front by the caller.
func basePool(pool *x509.CertPool) *x509.CertPool {
	if pool == nil {
		return x509.NewCertPool()
	}

	return pool.Clone()
}
