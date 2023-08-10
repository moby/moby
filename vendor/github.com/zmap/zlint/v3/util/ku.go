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
