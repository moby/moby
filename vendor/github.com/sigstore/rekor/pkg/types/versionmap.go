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
	"fmt"
	"sync"

	"github.com/blang/semver"
	"github.com/sigstore/rekor/pkg/internal/log"
)

// VersionEntryFactoryMap defines a map-like interface to find the correct implementation for a version string
// This could be a simple map[string][EntryFactory], or something more elegant (e.g. semver)
type VersionEntryFactoryMap interface {
	GetEntryFactory(string) (EntryFactory, error) // return the entry factory for the specified version
	SetEntryFactory(string, EntryFactory) error   // set the entry factory for the specified version
	Count() int                                   // return the count of entry factories currently in the map
	SupportedVersions() []string                  // return a list of versions currently stored in the map
}

// SemVerEntryFactoryMap implements a map that allows implementations to specify their supported versions using
// semver-compliant strings
type SemVerEntryFactoryMap struct {
	factoryMap map[string]EntryFactory

	sync.RWMutex
}

func NewSemVerEntryFactoryMap() VersionEntryFactoryMap {
	s := SemVerEntryFactoryMap{}
	s.factoryMap = make(map[string]EntryFactory)
	return &s
}

func (s *SemVerEntryFactoryMap) Count() int {
	return len(s.factoryMap)
}

func (s *SemVerEntryFactoryMap) GetEntryFactory(version string) (EntryFactory, error) {
	s.RLock()
	defer s.RUnlock()

	semverToMatch, err := semver.Parse(version)
	if err != nil {
		log.Logger.Error(err)
		return nil, err
	}

	// will return first function that matches
	for k, v := range s.factoryMap {
		semverRange, err := semver.ParseRange(k)
		if err != nil {
			log.Logger.Error(err)
			return nil, err
		}

		if semverRange(semverToMatch) {
			return v, nil
		}
	}
	return nil, fmt.Errorf("unable to locate entry for version %s", version)
}

func (s *SemVerEntryFactoryMap) SetEntryFactory(constraint string, ef EntryFactory) error {
	s.Lock()
	defer s.Unlock()

	if _, err := semver.ParseRange(constraint); err != nil {
		log.Logger.Error(err)
		return err
	}

	s.factoryMap[constraint] = ef
	return nil
}

func (s *SemVerEntryFactoryMap) SupportedVersions() []string {
	var versions []string
	for k := range s.factoryMap {
		versions = append(versions, k)
	}
	return versions
}
