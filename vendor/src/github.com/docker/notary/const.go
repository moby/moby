package notary

// application wide constants
const (
	// MinThreshold requires a minimum of one threshold for roles; currently we do not support a higher threshold
	MinThreshold = 1
	// PrivKeyPerms are the file permissions to use when writing private keys to disk
	PrivKeyPerms = 0700
	// PubCertPerms are the file permissions to use when writing public certificates to disk
	PubCertPerms = 0755
	// Sha256HexSize is how big a Sha256 hex is in number of characters
	Sha256HexSize = 64
	// TrustedCertsDir is the directory, under the notary repo base directory, where trusted certs are stored
	TrustedCertsDir = "trusted_certificates"
)
