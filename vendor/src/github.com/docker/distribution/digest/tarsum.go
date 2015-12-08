package digest

import (
	"fmt"

	"regexp"
)

// TarsumRegexp defines a regular expression to match tarsum identifiers.
var TarsumRegexp = regexp.MustCompile("tarsum(?:.[a-z0-9]+)?\\+[a-zA-Z0-9]+:[A-Fa-f0-9]+")

// TarsumRegexpCapturing defines a regular expression to match tarsum identifiers with
// capture groups corresponding to each component.
var TarsumRegexpCapturing = regexp.MustCompile("(tarsum)(.([a-z0-9]+))?\\+([a-zA-Z0-9]+):([A-Fa-f0-9]+)")

// TarSumInfo contains information about a parsed tarsum.
type TarSumInfo struct {
	// Version contains the version of the tarsum.
	Version string

	// Algorithm contains the algorithm for the final digest
	Algorithm string

	// Digest contains the hex-encoded digest.
	Digest string
}

// InvalidTarSumError provides informations about a TarSum that cannot be parsed
// by ParseTarSum.
type InvalidTarSumError string

func (e InvalidTarSumError) Error() string {
	return fmt.Sprintf("invalid tarsum: %q", string(e))
}

// ParseTarSum parses a tarsum string into its components of interest. For
// example, this method may receive the tarsum in the following format:
//
//		tarsum.v1+sha256:220a60ecd4a3c32c282622a625a54db9ba0ff55b5ba9c29c7064a2bc358b6a3e
//
// The function will return the following:
//
//		TarSumInfo{
//			Version: "v1",
//			Algorithm: "sha256",
//			Digest: "220a60ecd4a3c32c282622a625a54db9ba0ff55b5ba9c29c7064a2bc358b6a3e",
//		}
//
func ParseTarSum(tarSum string) (tsi TarSumInfo, err error) {
	components := TarsumRegexpCapturing.FindStringSubmatch(tarSum)

	if len(components) != 1+TarsumRegexpCapturing.NumSubexp() {
		return TarSumInfo{}, InvalidTarSumError(tarSum)
	}

	return TarSumInfo{
		Version:   components[3],
		Algorithm: components[4],
		Digest:    components[5],
	}, nil
}

// String returns the valid, string representation of the tarsum info.
func (tsi TarSumInfo) String() string {
	if tsi.Version == "" {
		return fmt.Sprintf("tarsum+%s:%s", tsi.Algorithm, tsi.Digest)
	}

	return fmt.Sprintf("tarsum.%s+%s:%s", tsi.Version, tsi.Algorithm, tsi.Digest)
}
