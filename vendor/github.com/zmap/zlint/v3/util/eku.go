package util

import "github.com/zmap/zcrypto/x509"

// HasEKU tests whether an Extended Key Usage (EKU) is present in a certificate.
func HasEKU(cert *x509.Certificate, eku x509.ExtKeyUsage) bool {
	for _, currentEku := range cert.ExtKeyUsage {
		if currentEku == eku {
			return true
		}
	}

	return false
}
