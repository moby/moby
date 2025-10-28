package tlsconfig

import "crypto/x509"

// SystemCertPool returns a copy of the system cert pool.
//
// Deprecated: use [x509.SystemCertPool] instead.
//
//go:fix inline
func SystemCertPool() (*x509.CertPool, error) {
	return x509.SystemCertPool()
}
