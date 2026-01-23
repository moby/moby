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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/pelletier/go-toml"
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
func (f FilterOptions) Empty() bool {
	return f.NameFilter == nil &&
		len(f.IncludeNames) == 0 &&
		len(f.ExcludeNames) == 0 &&
		len(f.IncludeSources) == 0 &&
		len(f.ExcludeSources) == 0
}

// AddProfile takes in a Profile and appends all Profile.LintNames
// into FilterOptions.IncludeNames.
func (f *FilterOptions) AddProfile(profile Profile) {
	if f.IncludeNames == nil {
		f.IncludeNames = make([]string, 0)
	}
	f.IncludeNames = append(f.IncludeNames, profile.LintNames...)
}

// Registry is an interface describing a collection of registered lints.
// A Registry instance can be given to zlint.LintCertificateEx() to control what
// lints are run for a given certificate.
//
// Typically users will interact with the global Registry returned by
// GlobalRegistry(), or a filtered Registry created by applying FilterOptions to
// the GlobalRegistry()'s Filter function.
type Registry interface { //nolint: interfacebloat // Somewhat unavoidable here.
	// Names returns a list of all of the lint names that have been registered
	// in string sorted order.
	Names() []string
	// Sources returns a SourceList of registered LintSources. The list is not
	// sorted but can be sorted by the caller with sort.Sort() if required.
	Sources() SourceList
	// @TODO
	DefaultConfiguration() ([]byte, error)
	// ByName returns a pointer to the registered lint with the given name, or nil
	// if there is no such lint registered in the registry.
	//
	// @deprecated - use CertificateLints instead.
	ByName(name string) *Lint
	// BySource returns a list of registered lints that have the same LintSource as
	// provided (or nil if there were no such lints in the registry).
	//
	// @deprecated - use CertificateLints instead.
	BySource(s LintSource) []*Lint
	// Filter returns a new Registry containing only lints that match the
	// FilterOptions criteria.
	Filter(opts FilterOptions) (Registry, error)
	// WriteJSON writes a description of each registered lint as
	// a JSON object, one object per line, to the provided writer.
	WriteJSON(w io.Writer)
	SetConfiguration(config Configuration)
	GetConfiguration() Configuration
	// CertificateLints returns an interface used to lookup CertificateLints.
	CertificateLints() CertificateLinterLookup
	// RevocationListLitns returns an interface used to lookup RevocationListLints.
	RevocationListLints() RevocationListLinterLookup
}

// registryImpl implements the Registry interface to provide a global collection
// of Lints that have been registered.
type registryImpl struct {
	certificateLints    certificateLinterLookupImpl
	revocationListLints revocationListLinterLookupImpl
	configuration       Configuration
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

// registerLint registers a lint to the registry.
//
// @deprecated - use registerCertificateLint instead.
func (r *registryImpl) register(l *Lint) error {
	if l == nil {
		return errNilLint
	}
	if l.Lint() == nil {
		return errNilLintPtr
	}

	return r.registerCertificateLint(l.toCertificateLint())
}

// registerCertificateLint registers a CertificateLint to the registry.
//
// An error is returned if the lint or lint's Lint pointer is nil, if the Lint
// has an empty Name or if the Name was previously registered.
func (r *registryImpl) registerCertificateLint(l *CertificateLint) error {
	if l == nil {
		return errNilLint
	}
	if l.Lint() == nil {
		return errNilLintPtr
	}
	return r.certificateLints.register(l, l.Name, l.Source)
}

// registerCertificateLint registers a CertificateLint to the registry.
//
// An error is returned if the lint or lint's Lint pointer is nil, if the Lint
// has an empty Name or if the Name was previously registered.
func (r *registryImpl) registerRevocationListLint(l *RevocationListLint) error {
	if l == nil {
		return errNilLint
	}
	if l.Lint() == nil {
		return errNilLintPtr
	}
	return r.revocationListLints.register(l, l.Name, l.Source)
}

// ByName returns the Lint previously registered under the given name with
// Register, or nil if no matching lint name has been registered.
//
// @deprecated - use r.CertificateLints.ByName() instead.
func (r *registryImpl) ByName(name string) *Lint {
	certificateLint := r.certificateLints.ByName(name)
	if certificateLint == nil {
		return nil
	}

	return certificateLint.toLint()
}

// Names returns a list of all of the lint names that have been registered
// in string sorted order.
func (r *registryImpl) Names() []string {
	var names []string
	names = append(names, r.certificateLints.lintNames...)
	names = append(names, r.revocationListLints.lintNames...)

	sort.Strings(names)
	return names
}

// BySource returns a list of registered lints that have the same LintSource as
// provided (or nil if there were no such lints).
//
// @deprecated use r.CertificateLints().BySource() instead.
func (r *registryImpl) BySource(s LintSource) []*Lint {
	var lints []*Lint

	certificateLints := r.certificateLints.BySource(s)
	for _, l := range certificateLints {
		if l == nil {
			continue
		}
		lints = append(lints, l.toLint())
	}

	return lints
}

// Sources returns a SourceList of registered LintSources. The list is not
// sorted but can be sorted by the caller with sort.Sort() if required.
func (r *registryImpl) Sources() SourceList {
	var sources SourceList

	sources = append(sources, r.certificateLints.Sources()...)
	sources = append(sources, r.revocationListLints.Sources()...)
	return sources
}

func (r *registryImpl) CertificateLints() CertificateLinterLookup {
	return &r.certificateLints
}

func (r *registryImpl) RevocationListLints() RevocationListLinterLookup {
	return &r.revocationListLints
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
		if l := r.certificateLints.ByName(n); l != nil {
			namesMap[n] = true
			continue
		}
		if l := r.revocationListLints.ByName(n); l != nil {
			namesMap[n] = true
			continue
		}
		return nil, fmt.Errorf("unknown lint name %q", n)
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
//
//	ExcludeSources > IncludeSources > NameFilter > ExcludeNames > IncludeNames
//
//nolint:cyclop
func (r *registryImpl) Filter(opts FilterOptions) (Registry, error) {
	// If there's no filtering to be done, return the existing Registry.
	if opts.Empty() {
		return r, nil
	}

	filteredRegistry := NewRegistry()
	filteredRegistry.SetConfiguration(r.configuration)

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
		var meta LintMetadata
		var registerFunc func() error

		if l := r.certificateLints.ByName(name); l != nil {
			meta = l.LintMetadata
			registerFunc = func() error {
				return filteredRegistry.registerCertificateLint(l)
			}
		} else if l := r.revocationListLints.ByName(name); l != nil {
			meta = l.LintMetadata
			registerFunc = func() error {
				return filteredRegistry.registerRevocationListLint(l)
			}
		}

		if sourceExcludes != nil && sourceExcludes[meta.Source] {
			continue
		}
		if sourceIncludes != nil && !sourceIncludes[meta.Source] {
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

		if err := registerFunc(); err != nil {
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
	for _, lint := range r.certificateLints.Lints() {
		//nolint:errchkjson
		_ = enc.Encode(lint)
	}

	for _, lint := range r.revocationListLints.Lints() {
		//nolint:errchkjson
		_ = enc.Encode(lint)
	}
}

func (r *registryImpl) SetConfiguration(cfg Configuration) {
	r.configuration = cfg
}

func (r *registryImpl) GetConfiguration() Configuration {
	return r.configuration
}

// DefaultConfiguration returns a serialized copy of the default configuration for ZLint.
//
// This is especially useful combined with the -exampleConfig CLI argument which prints this
// to stdout. In this way, operators can quickly see what lints are configurable and what their
// fields are without having to dig through documentation or, even worse, code.
func (r *registryImpl) DefaultConfiguration() ([]byte, error) {
	return r.defaultConfiguration(defaultGlobals)
}

// defaultConfiguration is abstracted out to a private function that takes in a slice of globals
// for the sake of making unit testing easier.
func (r *registryImpl) defaultConfiguration(globals []GlobalConfiguration) ([]byte, error) {
	configurables := map[string]interface{}{}
	for name, lint := range r.certificateLints.lintsByName {
		switch configurable := lint.Lint().(type) {
		case Configurable:
			configurables[name] = stripGlobalsFromExample(configurable.Configure())
		default:
		}
	}

	for name, lint := range r.revocationListLints.lintsByName {
		switch configurable := lint.Lint().(type) {
		case Configurable:
			configurables[name] = stripGlobalsFromExample(configurable.Configure())
		default:
		}
	}

	for _, config := range globals {
		switch config.(type) {
		case *Global:
			// We're just using stripGlobalsFromExample here as a convenient way to
			// recursively turn the `Global` struct type into a map.
			//
			// We have to do this because if we simply followed the pattern above and did...
			//
			//	configurables["Global"] = &Global{}
			//
			// ...then we would end up with a [Global] section in the resulting configuration,
			// which is not what we are looking for (we simply want it to be flattened out into
			// the top most context of the configuration file).
			for k, v := range stripGlobalsFromExample(config).(map[string]interface{}) {
				configurables[k] = v
			}
		default:
			configurables[config.namespace()] = config
		}

	}
	w := &bytes.Buffer{}
	err := toml.NewEncoder(w).Indentation("").CompactComments(true).Encode(configurables)
	if err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

// NewRegistry constructs a Registry implementation that can be used to register
// lints.
//
//nolint:revive
func NewRegistry() *registryImpl {
	registry := &registryImpl{
		certificateLints:    newCertificateLintLookup(),
		revocationListLints: newRevocationListLintLookup(),
	}
	registry.SetConfiguration(NewEmptyConfig())
	return registry
}

// globalRegistry is the Registry used by all loaded lints that call
// RegisterLint().
var globalRegistry = NewRegistry()

// RegisterLint must be called once for each lint to be executed. Normally,
// RegisterLint is called from the Go init() function of a lint implementation.
//
// IMPORTANT: RegisterLint will panic if given a nil lint, or a lint with a nil
// Lint pointer, or if the lint name matches a previously registered lint's
// name. These conditions all indicate a bug that should be addressed by a
// developer.
//
// @deprecated - use RegisterCertificateLint instead.
func RegisterLint(l *Lint) {
	RegisterCertificateLint(l.toCertificateLint())
}

// RegisterCertificateLint must be called once for each CertificateLint to be executed.
// Normally, RegisterCertificateLint is called from the Go init() function of a lint implementation.
//
// IMPORTANT: RegisterCertificateLint will panic if given a nil lint, or a lint
// with a nil Lint pointer, or if the lint name matches a previously registered
// lint's name. These conditions all indicate a bug that should be addressed by
// a developer.
func RegisterCertificateLint(l *CertificateLint) {
	if err := globalRegistry.registerCertificateLint(l); err != nil {
		panic(fmt.Sprintf("RegisterLint error: %v\n", err.Error()))
	}
}

// RegisterRevocationListLint must be called once for each RevocationListLint to be executed.
// Normally, RegisterRevocationListLint is called from the Go init() function of a lint implementation.
//
// IMPORTANT: RegisterRevocationListLint will panic if given a nil lint, or a
// lint with a nil Lint pointer, or if the lint name matches a previously
// registered lint's name. These conditions all indicate a bug that should be
// addressed by a developer.
func RegisterRevocationListLint(l *RevocationListLint) {
	// RegisterLint always sets initialize to true. It's assumed this is called by
	// the package init() functions and therefore must be doing the first
	// initialization of a lint.
	if err := globalRegistry.registerRevocationListLint(l); err != nil {
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
