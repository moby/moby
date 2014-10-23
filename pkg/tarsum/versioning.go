package tarsum

import (
	"errors"
	"strings"
)

// versioning of the TarSum algorithm
// based on the prefix of the hash used
// i.e. "tarsum+sha256:e58fcf7418d4390dec8e8fb69d88c06ec07039d651fedd3aa72af9972e7d046b"
type Version int

const (
	// Prefix of "tarsum"
	Version0 Version = iota
	// Prefix of "tarsum.dev"
	// NOTE: this variable will be of an unsettled next-version of the TarSum calculation
	VersionDev
)

// Get a list of all known tarsum Version
func GetVersions() []Version {
	v := []Version{}
	for k := range tarSumVersions {
		v = append(v, k)
	}
	return v
}

var tarSumVersions = map[Version]string{
	0: "tarsum",
	1: "tarsum.dev",
}

func (tsv Version) String() string {
	return tarSumVersions[tsv]
}

// GetVersionFromTarsum returns the Version from the provided string
func GetVersionFromTarsum(tarsum string) (Version, error) {
	tsv := tarsum
	if strings.Contains(tarsum, "+") {
		tsv = strings.SplitN(tarsum, "+", 2)[0]
	}
	for v, s := range tarSumVersions {
		if s == tsv {
			return v, nil
		}
	}
	return -1, ErrNotVersion
}

var (
	ErrNotVersion            = errors.New("string does not include a TarSum Version")
	ErrVersionNotImplemented = errors.New("TarSum Version is not yet implemented")
)
