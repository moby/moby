/*
 * ZLint Copyright 2021 Regents of the University of Michigan
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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// FilterOptions is a struct used by Registry.Filter to create a sub registry
// containing only lints that meet the filter options specified.
//
// Source based exclusion/inclusion is evaluated before Lint name based
// exclusion/inclusion. In both cases exclusion is processed before inclusion.
//
// Only one of NameFilter or IncludeNames/ExcludeNames can be provided at
// a time.
type FilterOptions struct {
	// NameFilter is a regexp used to filter lints by their name. It is mutually
	// exclusive with IncludeNames and ExcludeNames.
	NameFilter *regexp.Regexp
	// IncludeNames is a case sensitive list of lint names to include in the
	// registry being filtered.
	IncludeNames []string
	// ExcludeNames is a case sensitive list of lint names to exclude from the
	// registry being filtered.
	ExcludeNames []string
	// IncludeSource is a SourceList of LintSource's to be included in the
	// registry being filtered.
	IncludeSources SourceList
	// ExcludeSources is a SourceList of LintSources's to be excluded in the
	// registry being filtered.
	ExcludeSources SourceList
}

// Empty returns true if the FilterOptions is empty and does not specify any
// elements to filter by.
func (opts FilterOptions) Empty() bool {
	return opts.NameFilter == nil &&
		len(opts.IncludeNames) == 0 &&
		len(opts.ExcludeNames) == 0 &&
		len(opts.IncludeSources) == 0 &&
		len(opts.ExcludeSources) == 0
}

// Registry is an interface describing a collection of registered lints.
// A Registry instance can be given to zlint.LintCertificateEx() to control what
// lints are run for a given certificate.
//
// Typically users will interact with the global Registry returned by
// GlobalRegistry(), or a filtered Registry created by applying FilterOptions to
// the GlobalRegistry()'s Filter function.
type Registry interface {
	// Names returns a list of all of the lint names that have been registered
	// in string sorted order.
	Names() []string
	// Sources returns a SourceList of registered LintSources. The list is not
	// sorted but can be sorted by the caller with sort.Sort() if required.
	Sources() SourceList
	// ByName returns a pointer to the registered lint with the given name, or nil
	// if there is no such lint registered in the registry.
	ByName(name string) *Lint
	// BySource returns a list of registered lints that have the same LintSource as
	// provided (or nil if there were no such lints in the registry).
	BySource(s LintSource) []*Lint
	// Filter returns a new Registry containing only lints that match the
	// FilterOptions criteria.
	Filter(opts FilterOptions) (Registry, error)
	// WriteJSON writes a description of each registered lint as
	// a JSON object, one object per line, to the provided writer.
	WriteJSON(w io.Writer)
}

// registryImpl implements the Registry interface to provide a global collection
// of Lints that have been registered.
type registryImpl struct {
	sync.RWMutex
	// lintsByName is a map of all registered lints by name.
	lintsByName map[string]*Lint
	// lintNames is a sorted list of all of the registered lint names. It is
	// equivalent to collecting the keys from lintsByName into a slice and sorting
	// them lexicographically.
	lintNames []string
	// lintsBySource is a map of all registered lints by source category. Lints
	// are added to the lintsBySource map by RegisterLint.
	lintsBySource map[LintSource][]*Lint
}

var (
	// errNilLint is returned from registry.Register if the provided lint was nil.
	errNilLint = errors.New("can not register a nil lint")
	// errNilLintPtr is returned from registry.Register if the provided lint had
	// a nil Lint field.
	errNilLintPtr = errors.New("can not register a lint with a nil Lint pointer")
	// errEmptyName is returned from registry.Register if the provided lint had an
	// empty Name field.
	errEmptyName = errors.New("can not register a lint with an empty Name")
)

// errDuplicateName is returned from registry.Register if the provided lint had
// a Name field matching a lint that was previously registered.
type errDuplicateName struct {
	lintName string
}

func (e errDuplicateName) Error() string {
	return fmt.Sprintf(
		"can not register lint with name %q - it has already been registered",
		e.lintName)
}

// errBadInit is returned from registry.Register if the provided lint's
// Initialize function returned an error.
type errBadInit struct {
	lintName string
	err      error
}

func (e errBadInit) Error() string {
	return fmt.Sprintf(
		"failed to register lint with name %q - failed to Initialize: %q",
		e.lintName, e.err)
}

// register adds the provided lint to the Registry. If initialize is true then
// the lint's Initialize() function will be called before registering the lint.
//
// An error is returned if the lint or lint's Lint pointer is nil, if the Lint
// has an empty Name or if the Name was previously registered.
func (r *registryImpl) register(l *Lint, initialize bool) error {
	if l == nil {
		return errNilLint
	}
	if l.Lint == nil {
		return errNilLintPtr
	}
	if l.Name == "" {
		return errEmptyName
	}
	if existing := r.ByName(l.Name); existing != nil {
		return &errDuplicateName{l.Name}
	}
	if initialize {
		if err := l.Lint.Initialize(); err != nil {
			return &errBadInit{l.Name, err}
		}
	}
	r.Lock()
	defer r.Unlock()
	r.lintNames = append(r.lintNames, l.Name)
	r.lintsByName[l.Name] = l
	r.lintsBySource[l.Source] = append(r.lintsBySource[l.Source], l)
	sort.Strings(r.lintNames)
	return nil
}

// ByName returns the Lint previously registered under the given name with
// Register, or nil if no matching lint name has been registered.
func (r *registryImpl) ByName(name string) *Lint {
	r.RLock()
	defer r.RUnlock()
	return r.lintsByName[name]
}

// Names returns a list of all of the lint names that have been registered
// in string sorted order.
func (r *registryImpl) Names() []string {
	r.RLock()
	defer r.RUnlock()
	return r.lintNames
}

// BySource returns a list of registered lints that have the same LintSource as
// provided (or nil if there were no such lints).
func (r *registryImpl) BySource(s LintSource) []*Lint {
	r.RLock()
	defer r.RUnlock()
	return r.lintsBySource[s]
}

// Sources returns a SourceList of registered LintSources. The list is not
// sorted but can be sorted by the caller with sort.Sort() if required.
func (r *registryImpl) Sources() SourceList {
	r.RLock()
	defer r.RUnlock()
	var results SourceList
	for k := range r.lintsBySource {
		results = append(results, k)
	}
	return results
}

// lintNamesToMap converts a list of lit names into a bool hashmap useful for
// filtering. If any of the lint names are not known by the registry an error is
// returned.
func (r *registryImpl) lintNamesToMap(names []string) (map[string]bool, error) {
	if len(names) == 0 {
		return nil, nil
	}

	namesMap := make(map[string]bool, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if l := r.ByName(n); l == nil {
			return nil, fmt.Errorf("unknown lint name %q", n)
		}
		namesMap[n] = true
	}
	return namesMap, nil
}

func sourceListToMap(sources SourceList) map[LintSource]bool {
	if len(sources) == 0 {
		return nil
	}
	sourceMap := make(map[LintSource]bool, len(sources))
	for _, s := range sources {
		sourceMap[s] = true
	}
	return sourceMap
}

// Filter creates a new Registry with only the lints that meet the FilterOptions
// criteria included.
//
// FilterOptions are applied in the following order of precedence:
//   ExcludeSources > IncludeSources > NameFilter > ExcludeNames > IncludeNames
func (r *registryImpl) Filter(opts FilterOptions) (Registry, error) {
	// If there's no filtering to be done, return the existing Registry.
	if opts.Empty() {
		return r, nil
	}

	filteredRegistry := NewRegistry()

	sourceExcludes := sourceListToMap(opts.ExcludeSources)
	sourceIncludes := sourceListToMap(opts.IncludeSources)

	nameExcludes, err := r.lintNamesToMap(opts.ExcludeNames)
	if err != nil {
		return nil, err
	}
	nameIncludes, err := r.lintNamesToMap(opts.IncludeNames)
	if err != nil {
		return nil, err
	}

	if opts.NameFilter != nil && (len(nameExcludes) != 0 || len(nameIncludes) != 0) {
		return nil, errors.New(
			"FilterOptions.NameFilter cannot be used at the same time as " +
				"FilterOptions.ExcludeNames or FilterOptions.IncludeNames")
	}

	for _, name := range r.Names() {
		l := r.ByName(name)

		if sourceExcludes != nil && sourceExcludes[l.Source] {
			continue
		}
		if sourceIncludes != nil && !sourceIncludes[l.Source] {
			continue
		}
		if opts.NameFilter != nil && !opts.NameFilter.MatchString(name) {
			continue
		}
		if nameExcludes != nil && nameExcludes[name] {
			continue
		}
		if nameIncludes != nil && !nameIncludes[name] {
			continue
		}

		// when adding lints to a filtered registry we do not want Initialize() to
		// be called a second time, so provide false as the initialize argument.
		if err := filteredRegistry.register(l, false); err != nil {
			return nil, err
		}
	}

	return filteredRegistry, nil
}

// WriteJSON writes a description of each registered lint as
// a JSON object, one object per line, to the provided writer.
func (r *registryImpl) WriteJSON(w io.Writer) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, name := range r.Names() {
		_ = enc.Encode(r.ByName(name))
	}
}

// NewRegistry constructs a Registry implementation that can be used to register
// lints.
func NewRegistry() *registryImpl {
	return &registryImpl{
		lintsByName:   make(map[string]*Lint),
		lintsBySource: make(map[LintSource][]*Lint),
	}
}

// globalRegistry is the Registry used by all loaded lints that call
// RegisterLint().
var globalRegistry *registryImpl = NewRegistry()

// RegisterLint must be called once for each lint to be executed. Normally,
// RegisterLint is called from the Go init() function of a lint implementation.
//
// RegsterLint will call l.Lint's Initialize() function as part of the
// registration process.
//
// IMPORTANT: RegisterLint will panic if given a nil lint, or a lint with a nil
// Lint pointer, or if the lint's Initialize function errors, or if the lint
// name matches a previously registered lint's name. These conditions all
// indicate a bug that should be addressed by a developer.
func RegisterLint(l *Lint) {
	// RegisterLint always sets initialize to true. It's assumed this is called by
	// the package init() functions and therefore must be doing the first
	// initialization of a lint.
	if err := globalRegistry.register(l, true); err != nil {
		panic(fmt.Sprintf("RegisterLint error: %v\n", err.Error()))
	}
}

// GlobalRegistry is the Registry used by RegisterLint and contains all of the
// lints that are loaded.
//
// If you want to run only a subset of the globally registered lints use
// GloablRegistry().Filter with FilterOptions to create a filtered
// Registry.
func GlobalRegistry() Registry {
	return globalRegistry
}
