//
// Copyright 2021 The Sigstore Authors.
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

package types

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/sigstore/rekor/pkg/generated/models"
)

// TypeMap stores mapping between type strings and entry constructors
// entries are written once at process initialization and read for each transaction, so we use
// sync.Map which is optimized for this case
var TypeMap sync.Map

// RekorType is the base struct that is embedded in all type implementations
type RekorType struct {
	Kind       string                 // this is the unique string that identifies the type
	VersionMap VersionEntryFactoryMap // this maps the supported versions to implementation
}

// TypeImpl is implemented by all types to support the polymorphic conversion of abstract
// proposed entry to a working implementation for the versioned type requested, if supported
type TypeImpl interface {
	CreateProposedEntry(context.Context, string, ArtifactProperties) (models.ProposedEntry, error)
	DefaultVersion() string
	SupportedVersions() []string
	IsSupportedVersion(string) bool
	UnmarshalEntry(pe models.ProposedEntry) (EntryImpl, error)
}

// VersionedUnmarshal extracts the correct implementing factory function from the type's version map,
// creates an entry of that versioned type and then calls that versioned type's unmarshal method
func (rt *RekorType) VersionedUnmarshal(pe models.ProposedEntry, version string) (EntryImpl, error) {
	ef, err := rt.VersionMap.GetEntryFactory(version)
	if err != nil {
		return nil, fmt.Errorf("%s implementation for version '%v' not found: %w", rt.Kind, version, err)
	}
	entry := ef()
	if entry == nil {
		return nil, errors.New("failure generating object")
	}
	if pe == nil {
		return entry, nil
	}
	return entry, entry.Unmarshal(pe)
}

// SupportedVersions returns a list of versions of this type that can be currently entered into the log
func (rt *RekorType) SupportedVersions() []string {
	return rt.VersionMap.SupportedVersions()
}

// IsSupportedVersion returns true if the version can be inserted into the log, and false if not
func (rt *RekorType) IsSupportedVersion(proposedVersion string) bool {
	return slices.Contains(rt.SupportedVersions(), proposedVersion)
}

// ListImplementedTypes returns a list of all type strings currently known to
// be implemented
func ListImplementedTypes() []string {
	retVal := []string{}
	TypeMap.Range(func(k interface{}, v interface{}) bool {
		tf := v.(func() TypeImpl)
		for _, verStr := range tf().SupportedVersions() {
			retVal = append(retVal, fmt.Sprintf("%v:%v", k.(string), verStr))
		}
		return true
	})
	return retVal
}
