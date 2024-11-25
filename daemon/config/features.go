package config

import "errors"

// DefaultFeatures maps features the engine supports to
// their default value.
// When a features becomes on by default, update this.
var DefaultFeatures = map[string]bool{
	"containerd-snapshotter": false,
}

// PopulateFeatures adds all the features the engine is aware of
// to features, set to their current default value.
//
// This is used to populate the daemon config features map, so that
// instead of this map not including features that are off by default
// and not enabled – or on by default and not disabled – it contains
// all features the engine is are of.
// This prevents older clients from assuming that engine features are
// disabled when communicating with newer daemons where the feature is
// now enabled by default.
func PopulateFeatures(features map[string]bool) error {
	if features == nil {
		return errors.New("features cannot be nil")
	}
	if DefaultFeatures == nil {
		return errors.New("DefaultFeatures cannot be nil")
	}

	for k, v := range DefaultFeatures {
		if _, exists := features[k]; !exists {
			features[k] = v
		}
	}

	return nil
}
