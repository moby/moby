/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"github.com/docker/distribution/reference"
)

// NormalizeImageRef normalizes the image reference following the docker convention. This is added
// mainly for backward compatibility.
// The reference returned can only be either tagged or digested. For reference contains both tag
// and digest, the function returns digested reference, e.g. docker.io/library/busybox:latest@
// sha256:7cc4b5aefd1d0cadf8d97d4350462ba51c694ebca145b08d7d41b41acc8db5aa will be returned as
// docker.io/library/busybox@sha256:7cc4b5aefd1d0cadf8d97d4350462ba51c694ebca145b08d7d41b41acc8db5aa.
func NormalizeImageRef(ref string) (reference.Named, error) {
	named, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return nil, err
	}
	if _, ok := named.(reference.NamedTagged); ok {
		if canonical, ok := named.(reference.Canonical); ok {
			// The reference is both tagged and digested, only
			// return digested.
			newNamed, err := reference.WithName(canonical.Name())
			if err != nil {
				return nil, err
			}
			newCanonical, err := reference.WithDigest(newNamed, canonical.Digest())
			if err != nil {
				return nil, err
			}
			return newCanonical, nil
		}
	}
	return reference.TagNameOnly(named), nil
}
