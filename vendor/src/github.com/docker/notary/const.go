package notary

import (
	"time"
)

// application wide constants
const (
	// MaxDownloadSize is the maximum size we'll download for metadata if no limit is given
	MaxDownloadSize int64 = 100 << 20
	// MaxTimestampSize is the maximum size of timestamp metadata - 1MiB.
	MaxTimestampSize int64 = 1 << 20
	// MinRSABitSize is the minimum bit size for RSA keys allowed in notary
	MinRSABitSize = 2048
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
	// PrivDir is the directory, under the notary repo base directory, where private keys are stored
	PrivDir = "private"
	// RootKeysSubdir is the subdirectory under PrivDir where root private keys are stored
	RootKeysSubdir = "root_keys"
	// NonRootKeysSubdir is the subdirectory under PrivDir where non-root private keys are stored
	NonRootKeysSubdir = "tuf_keys"

	// Day is a duration of one day
	Day  = 24 * time.Hour
	Year = 365 * Day

	// NotaryRootExpiry is the duration representing the expiry time of the Root role
	NotaryRootExpiry      = 10 * Year
	NotaryTargetsExpiry   = 3 * Year
	NotarySnapshotExpiry  = 3 * Year
	NotaryTimestampExpiry = 14 * Day
)

// NotaryDefaultExpiries is the construct used to configure the default expiry times of
// the various role files.
var NotaryDefaultExpiries = map[string]time.Duration{
	"root":      NotaryRootExpiry,
	"targets":   NotaryTargetsExpiry,
	"snapshot":  NotarySnapshotExpiry,
	"timestamp": NotaryTimestampExpiry,
}
