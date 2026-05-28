/*
 * ZLint Copyright 2023 Regents of the University of Michigan
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy
 * of the License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
 * implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

package lint

import (
	"sort"
	"sync"
)

var (
	// Verify that the interface holds
	_ linterLookup               = &linterLookupImpl{}
	_ CertificateLinterLookup    = &certificateLinterLookupImpl{}
	_ RevocationListLinterLookup = &revocationListLinterLookupImpl{}
)

type linterLookup interface {
	// Names returns a list of all lint names that have been registered.
	// The returned list is sorted by lexicographical ordering.
	Names() []string
	// Sources returns a SourceList of registered LintSources. The list is not
	// sorted but can be sorted by the caller with sort.Sort() if required.
	Sources() SourceList
}

type linterLookupImpl struct {
	sync.RWMutex
	// lintNames is a sorted list of all registered lint names. It is
	// equivalent to collecting the keys from lintsByName into a slice and sorting
	// them lexicographically.
	lintNames []string
	sources   map[LintSource]struct{}
}

// Names returns the list of lint names registered for the lint type T.
func (lookup *linterLookupImpl) Names() []string {
	lookup.RLock()
	defer lookup.RUnlock()
	return lookup.lintNames
}

// Sources returns a SourceList of registered LintSources. The list is not
// sorted but can be sorted by the caller with sort.Sort() if required.
func (lookup *linterLookupImpl) Sources() SourceList {
	lookup.RLock()
	defer lookup.RUnlock()
	var list SourceList
	for lintSource := range lookup.sources {
		list = append(list, lintSource)
	}
	return list
}

func newLinterLookup() linterLookupImpl {
	return linterLookupImpl{
		lintNames: make([]string, 0),
		sources:   map[LintSource]struct{}{},
	}
}

// CertificateLinterLookup is an interface describing how registered certificate lints can be looked up.
type CertificateLinterLookup interface {
	linterLookup
	// ByName returns a pointer to the registered lint with the given name, or nil
	// if there is no such lint registered in the registry.
	ByName(name string) *CertificateLint
	// BySource returns a list of registered lints that have the same LintSource as
	// provided (or nil if there were no such lints in the registry).
	BySource(s LintSource) []*CertificateLint
	// Lints returns a list of all the lints registered.
	Lints() []*CertificateLint
}

type certificateLinterLookupImpl struct {
	linterLookupImpl
	// lintsByName is a map of all registered lints by name.
	lintsByName   map[string]*CertificateLint
	lintsBySource map[LintSource][]*CertificateLint
	lints         []*CertificateLint
}

// ByName returns the Lint previously registered under the given name with
// Register, or nil if no matching lint name has been registered.
func (lookup *certificateLinterLookupImpl) ByName(name string) *CertificateLint {
	lookup.RLock()
	defer lookup.RUnlock()
	return lookup.lintsByName[name]
}

// BySource returns a list of registered lints that have the same LintSource as
// provided (or nil if there were no such lints).
func (lookup *certificateLinterLookupImpl) BySource(s LintSource) []*CertificateLint {
	lookup.RLock()
	defer lookup.RUnlock()
	return lookup.lintsBySource[s]
}

// Lints returns a list of registered lints.
func (lookup *certificateLinterLookupImpl) Lints() []*CertificateLint {
	lookup.RLock()
	defer lookup.RUnlock()
	return lookup.lints
}

func (lookup *certificateLinterLookupImpl) register(lint *CertificateLint, name string, source LintSource) error {
	if name == "" {
		return errEmptyName
	}
	lookup.RLock()
	defer lookup.RUnlock()

	if existing := lookup.lintsByName[name]; existing != nil {
		return &errDuplicateName{name}
	}
	lookup.lints = append(lookup.lints, lint)
	lookup.lintNames = append(lookup.lintNames, name)
	lookup.lintsByName[name] = lint

	lookup.sources[source] = struct{}{}
	lookup.lintsBySource[source] = append(lookup.lintsBySource[source], lint)
	sort.Strings(lookup.lintNames)
	return nil
}

func newCertificateLintLookup() certificateLinterLookupImpl {
	return certificateLinterLookupImpl{
		linterLookupImpl: newLinterLookup(),
		lintsByName:      make(map[string]*CertificateLint),
		lintsBySource:    make(map[LintSource][]*CertificateLint),
		lints:            make([]*CertificateLint, 0),
	}
}

// RevocationListLinterLookup is an interface describing how registered revocation list lints can be looked up.
type RevocationListLinterLookup interface {
	linterLookup
	// ByName returns a pointer to the registered lint with the given name, or nil
	// if there is no such lint registered in the registry.
	ByName(name string) *RevocationListLint
	// BySource returns a list of registered lints that have the same LintSource as
	// provided (or nil if there were no such lints in the registry).
	BySource(s LintSource) []*RevocationListLint
	// Lints returns a list of all the lints registered.
	Lints() []*RevocationListLint
}

type revocationListLinterLookupImpl struct {
	linterLookupImpl
	// lintsByName is a map of all registered lints by name.
	lintsByName   map[string]*RevocationListLint
	lintsBySource map[LintSource][]*RevocationListLint
	lints         []*RevocationListLint
}

// ByName returns the Lint previously registered under the given name with
// Register, or nil if no matching lint name has been registered.
func (lookup *revocationListLinterLookupImpl) ByName(name string) *RevocationListLint {
	lookup.RLock()
	defer lookup.RUnlock()
	return lookup.lintsByName[name]
}

// BySource returns a list of registered lints that have the same LintSource as
// provided (or nil if there were no such lints).
func (lookup *revocationListLinterLookupImpl) BySource(s LintSource) []*RevocationListLint {
	lookup.RLock()
	defer lookup.RUnlock()
	return lookup.lintsBySource[s]
}

// Lints returns a list of registered lints.
func (lookup *revocationListLinterLookupImpl) Lints() []*RevocationListLint {
	lookup.RLock()
	defer lookup.RUnlock()
	return lookup.lints
}

func (lookup *revocationListLinterLookupImpl) register(lint *RevocationListLint, name string, source LintSource) error {
	if name == "" {
		return errEmptyName
	}
	lookup.RLock()
	defer lookup.RUnlock()

	if existing := lookup.lintsByName[name]; existing != nil {
		return &errDuplicateName{name}
	}
	lookup.lints = append(lookup.lints, lint)
	lookup.lintNames = append(lookup.lintNames, name)
	lookup.lintsByName[name] = lint

	lookup.sources[source] = struct{}{}
	lookup.lintsBySource[source] = append(lookup.lintsBySource[source], lint)
	sort.Strings(lookup.lintNames)
	return nil
}

func newRevocationListLintLookup() revocationListLinterLookupImpl {
	return revocationListLinterLookupImpl{
		linterLookupImpl: newLinterLookup(),
		lintsByName:      make(map[string]*RevocationListLint),
		lintsBySource:    make(map[LintSource][]*RevocationListLint),
		lints:            make([]*RevocationListLint, 0),
	}
}
