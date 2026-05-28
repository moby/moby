// Copyright 2016 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package convert

import (
	"fmt"

	"github.com/containernetworking/cni/pkg/types"
)

// ConvertFn should convert from the given arbitrary Result type into a
// Result implementing CNI specification version passed in toVersion.
// The function is guaranteed to be passed a Result type matching the
// fromVersion it was registered with, and is guaranteed to be
// passed a toVersion matching one of the toVersions it was registered with.
type ConvertFn func(from types.Result, toVersion string) (types.Result, error)

type converter struct {
	// fromVersion is the CNI Result spec version that convertFn accepts
	fromVersion string
	// toVersions is a list of versions that convertFn can convert to
	toVersions []string
	convertFn  ConvertFn
}

var converters []*converter

func findConverter(fromVersion, toVersion string) *converter {
	for _, c := range converters {
		if c.fromVersion == fromVersion {
			for _, v := range c.toVersions {
				if v == toVersion {
					return c
				}
			}
		}
	}
	return nil
}

// Convert converts a CNI Result to the requested CNI specification version,
// or returns an error if the conversion could not be performed or failed
func Convert(from types.Result, toVersion string) (types.Result, error) {
	if toVersion == "" {
		toVersion = "0.1.0"
	}

	fromVersion := from.Version()

	// Shortcut for same version
	if fromVersion == toVersion {
		return from, nil
	}

	// Otherwise find the right converter
	c := findConverter(fromVersion, toVersion)
	if c == nil {
		return nil, fmt.Errorf("no converter for CNI result version %s to %s",
			fromVersion, toVersion)
	}
	return c.convertFn(from, toVersion)
}

// RegisterConverter registers a CNI Result converter. SHOULD NOT BE CALLED
// EXCEPT FROM CNI ITSELF.
func RegisterConverter(fromVersion string, toVersions []string, convertFn ConvertFn) {
	// Make sure there is no converter already registered for these
	// from and to versions
	for _, v := range toVersions {
		if findConverter(fromVersion, v) != nil {
			panic(fmt.Sprintf("converter already registered for %s to %s",
				fromVersion, v))
		}
	}
	converters = append(converters, &converter{
		fromVersion: fromVersion,
		toVersions:  toVersions,
		convertFn:   convertFn,
	})
}
