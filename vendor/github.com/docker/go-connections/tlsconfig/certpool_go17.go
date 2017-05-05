// +build go1.7

package tlsconfig

import (
	"crypto/x509"
	"runtime"

	"github.com/Sirupsen/logrus"
)

// SystemCertPool returns a copy of the system cert pool,
// returns an error if failed to load or empty pool on windows.
func SystemCertPool() (*x509.CertPool, error) {
	certpool, err := x509.SystemCertPool()
	if err != nil && runtime.GOOS == "windows" {
		logrus.Infof("Unable to use system certificate pool: %v", err)
		return x509.NewCertPool(), nil
	}
	return certpool, err
}
