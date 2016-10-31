// +build !go1.7

package tlsconfig

import (
	"crypto/x509"

	"github.com/Sirupsen/logrus"
)

// SystemCertPool returns an new empty cert pool,
// accessing system cert pool is supported in go 1.7
func SystemCertPool() (*x509.CertPool, error) {
	logrus.Warn("Unable to use system certificate pool: requires building with go 1.7 or later")
	return x509.NewCertPool(), nil
}
