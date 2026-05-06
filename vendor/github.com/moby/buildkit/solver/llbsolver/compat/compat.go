package compat

import (
	"slices"

	"github.com/pkg/errors"
)

const (
	CompatibilityVersion013     = 10
	CompatibilityVersion015     = 20
	CompatibilityVersionCurrent = CompatibilityVersion015
)

// JobValueKey is the key used to store the compatibility version on a solver
// job via Job.SetValue/EachValue.
const JobValueKey = "llb.compatibilityversion"

var supportedCompatibilityVersions = []int{
	CompatibilityVersion013,
	CompatibilityVersion015,
}

func SupportedCompatibilityVersions() []int {
	return slices.Clone(supportedCompatibilityVersions)
}

func ValidateCompatibilityVersion(version int) error {
	if slices.Contains(supportedCompatibilityVersions, version) {
		return nil
	}
	if version > CompatibilityVersionCurrent {
		return errors.Errorf("unsupported compatibility-version %d: upgrade buildkit (max supported: %d)", version, CompatibilityVersionCurrent)
	}
	return errors.Errorf("unsupported compatibility-version %d (supported: %v)", version, supportedCompatibilityVersions)
}
