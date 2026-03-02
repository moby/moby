package util

import "github.com/zmap/zcrypto/x509"

var (
	// KeyUsageToString maps an x509.KeyUsage bitmask to its name.
	KeyUsageToString = map[x509.KeyUsage]string{
		x509.KeyUsageDigitalSignature:  "KeyUsageDigitalSignature",
		x509.KeyUsageContentCommitment: "KeyUsageContentCommitment",
		x509.KeyUsageKeyEncipherment:   "KeyUsageKeyEncipherment",
		x509.KeyUsageDataEncipherment:  "KeyUsageDataEncipherment",
		x509.KeyUsageKeyAgreement:      "KeyUsageKeyAgreement",
		x509.KeyUsageCertSign:          "KeyUsageCertSign",
		x509.KeyUsageCRLSign:           "KeyUsageCRLSign",
		x509.KeyUsageEncipherOnly:      "KeyUsageEncipherOnly",
		x509.KeyUsageDecipherOnly:      "KeyUsageDecipherOnly",
	}
)

// HasKeyUsageOID returns whether-or-not the OID 2.5.29.15 is present in the given certificate's extensions.
func HasKeyUsageOID(c *x509.Certificate) bool {
	return IsExtInCert(c, KeyUsageOID)
}

// HasKeyUsage returns whether-or-not the given x509.KeyUsage is present within the
// given certificate's KeyUsage bitmap. The certificate, however, is NOT checked for
// whether-or-not it actually has a key usage OID. If you wish to check for the presence
// of the key usage OID, please use HasKeyUsageOID.
func HasKeyUsage(c *x509.Certificate, usage x509.KeyUsage) bool {
	return KeyUsageIsPresent(c.KeyUsage, usage)
}

// KeyUsageIsPresent checks the provided bitmap (keyUsages) for presence of the provided x509.KeyUsage.
func KeyUsageIsPresent(keyUsages x509.KeyUsage, usage x509.KeyUsage) bool {
	return keyUsages&usage != 0
}
