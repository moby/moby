/*
Copyright (c) the purl authors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

// Package packageurl implements the package-url spec
package packageurl

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var (
	// QualifierKeyPattern describes a valid qualifier key:
	//
	// - The key must be composed only of ASCII letters and numbers, '.',
	//   '-' and '_' (period, dash and underscore).
	// - A key cannot start with a number.
	QualifierKeyPattern = regexp.MustCompile(`^[A-Za-z\.\-_][0-9A-Za-z\.\-_]*$`)
)

// These are the known purl types as defined in the spec. Some of these require
// special treatment during parsing.
// https://github.com/package-url/purl-spec#known-purl-types
var (
	// TypeBitbucket is a pkg:bitbucket purl.
	TypeBitbucket = "bitbucket"
	// TypeCocoapods is a pkg:cocoapods purl.
	TypeCocoapods = "cocoapods"
	// TypeCargo is a pkg:cargo purl.
	TypeCargo = "cargo"
	// TypeComposer is a pkg:composer purl.
	TypeComposer = "composer"
	// TypeConan is a pkg:conan purl.
	TypeConan = "conan"
	// TypeConda is a pkg:conda purl.
	TypeConda = "conda"
	// TypeCran is a pkg:cran purl.
	TypeCran = "cran"
	// TypeDebian is a pkg:deb purl.
	TypeDebian = "deb"
	// TypeDocker is a pkg:docker purl.
	TypeDocker = "docker"
	// TypeGem is a pkg:gem purl.
	TypeGem = "gem"
	// TypeGeneric is a pkg:generic purl.
	TypeGeneric = "generic"
	// TypeGithub is a pkg:github purl.
	TypeGithub = "github"
	// TypeGolang is a pkg:golang purl.
	TypeGolang = "golang"
	// TypeHackage is a pkg:hackage purl.
	TypeHackage = "hackage"
	// TypeHex is a pkg:hex purl.
	TypeHex = "hex"
	// TypeMaven is a pkg:maven purl.
	TypeMaven = "maven"
	// TypeNPM is a pkg:npm purl.
	TypeNPM = "npm"
	// TypeNuget is a pkg:nuget purl.
	TypeNuget = "nuget"
	// TypeOCI is a pkg:oci purl
	TypeOCI = "oci"
	// TypePyPi is a pkg:pypi purl.
	TypePyPi = "pypi"
	// TypeRPM is a pkg:rpm purl.
	TypeRPM = "rpm"
	// TypeSwift is pkg:swift purl
	TypeSwift = "swift"
)

// Qualifier represents a single key=value qualifier in the package url
type Qualifier struct {
	Key   string
	Value string
}

func (q Qualifier) String() string {
	// A value must be a percent-encoded string
	return fmt.Sprintf("%s=%s", q.Key, url.PathEscape(q.Value))
}

// Qualifiers is a slice of key=value pairs, with order preserved as it appears
// in the package URL.
type Qualifiers []Qualifier

// QualifiersFromMap constructs a Qualifiers slice from a string map. To get a
// deterministic qualifier order (despite maps not providing any iteration order
// guarantees) the returned Qualifiers are sorted in increasing order of key.
func QualifiersFromMap(mm map[string]string) Qualifiers {
	q := Qualifiers{}

	for k, v := range mm {
		q = append(q, Qualifier{Key: k, Value: v})
	}

	// sort for deterministic qualifier order
	sort.Slice(q, func(i int, j int) bool { return q[i].Key < q[j].Key })

	return q
}

// Map converts a Qualifiers struct to a string map.
func (qq Qualifiers) Map() map[string]string {
	m := make(map[string]string)

	for i := 0; i < len(qq); i++ {
		k := qq[i].Key
		v := qq[i].Value
		m[k] = v
	}

	return m
}

func (qq Qualifiers) String() string {
	var kvPairs []string
	for _, q := range qq {
		kvPairs = append(kvPairs, q.String())
	}
	return strings.Join(kvPairs, "&")
}

// PackageURL is the struct representation of the parts that make a package url
type PackageURL struct {
	Type       string
	Namespace  string
	Name       string
	Version    string
	Qualifiers Qualifiers
	Subpath    string
}

// NewPackageURL creates a new PackageURL struct instance based on input
func NewPackageURL(purlType, namespace, name, version string,
	qualifiers Qualifiers, subpath string) *PackageURL {

	return &PackageURL{
		Type:       purlType,
		Namespace:  namespace,
		Name:       name,
		Version:    version,
		Qualifiers: qualifiers,
		Subpath:    subpath,
	}
}

// ToString returns the human-readable instance of the PackageURL structure.
// This is the literal purl as defined by the spec.
func (p *PackageURL) ToString() string {
	// Start with the type and a colon
	purl := fmt.Sprintf("pkg:%s/", p.Type)
	// Add namespaces if provided
	if p.Namespace != "" {
		var ns []string
		for _, item := range strings.Split(p.Namespace, "/") {
			ns = append(ns, url.QueryEscape(item))
		}
		purl = purl + strings.Join(ns, "/") + "/"
	}
	// The name is always required and must be a percent-encoded string
	// Use url.QueryEscape instead of PathEscape, as it handles @ signs
	purl = purl + url.QueryEscape(p.Name)
	// If a version is provided, add it after the at symbol
	if p.Version != "" {
		// A name must be a percent-encoded string
		purl = purl + "@" + url.PathEscape(p.Version)
	}

	// Iterate over qualifiers and make groups of key=value
	var qualifiers []string
	for _, q := range p.Qualifiers {
		qualifiers = append(qualifiers, q.String())
	}
	// If there are one or more key=value pairs, append on the package url
	if len(qualifiers) != 0 {
		purl = purl + "?" + strings.Join(qualifiers, "&")
	}
	// Add a subpath if available
	if p.Subpath != "" {
		purl = purl + "#" + p.Subpath
	}
	return purl
}

func (p PackageURL) String() string {
	return p.ToString()
}

// FromString parses a valid package url string into a PackageURL structure
func FromString(purl string) (PackageURL, error) {
	initialIndex := strings.Index(purl, "#")
	// Start with purl being stored in the remainder
	remainder := purl
	substring := ""
	if initialIndex != -1 {
		initialSplit := strings.SplitN(purl, "#", 2)
		remainder = initialSplit[0]
		rightSide := initialSplit[1]
		rightSide = strings.TrimLeft(rightSide, "/")
		rightSide = strings.TrimRight(rightSide, "/")
		var rightSides []string

		for _, item := range strings.Split(rightSide, "/") {
			item = strings.Replace(item, ".", "", -1)
			item = strings.Replace(item, "..", "", -1)
			if item != "" {
				i, err := url.PathUnescape(item)
				if err != nil {
					return PackageURL{}, fmt.Errorf("failed to unescape path: %s", err)
				}
				rightSides = append(rightSides, i)
			}
		}
		substring = strings.Join(rightSides, "/")
	}
	qualifiers := Qualifiers{}
	index := strings.LastIndex(remainder, "?")
	// If we don't have anything to split then return an empty result
	if index != -1 {
		qualifier := remainder[index+1:]
		for _, item := range strings.Split(qualifier, "&") {
			kv := strings.Split(item, "=")
			key := strings.ToLower(kv[0])
			key, err := url.PathUnescape(key)
			if err != nil {
				return PackageURL{}, fmt.Errorf("failed to unescape qualifier key: %s", err)
			}
			if !validQualifierKey(key) {
				return PackageURL{}, fmt.Errorf("invalid qualifier key: '%s'", key)
			}
			// TODO
			//  - If the `key` is `checksums`, split the `value` on ',' to create
			//    a list of `checksums`
			if kv[1] == "" {
				continue
			}
			value, err := url.PathUnescape(kv[1])
			if err != nil {
				return PackageURL{}, fmt.Errorf("failed to unescape qualifier value: %s", err)
			}
			qualifiers = append(qualifiers, Qualifier{key, value})
		}
		remainder = remainder[:index]
	}

	nextSplit := strings.SplitN(remainder, ":", 2)
	if len(nextSplit) != 2 || nextSplit[0] != "pkg" {
		return PackageURL{}, errors.New("scheme is missing")
	}
	// leading slashes after pkg: are to be ignored (pkg://maven is
	// equivalent to pkg:maven)
	remainder = strings.TrimLeft(nextSplit[1], "/")

	nextSplit = strings.SplitN(remainder, "/", 2)
	if len(nextSplit) != 2 {
		return PackageURL{}, errors.New("type is missing")
	}
	// purl type is case-insensitive, canonical form is lower-case
	purlType := strings.ToLower(nextSplit[0])
	remainder = nextSplit[1]

	index = strings.LastIndex(remainder, "/")
	name := typeAdjustName(purlType, remainder[index+1:])
	version := ""

	atIndex := strings.Index(name, "@")
	if atIndex != -1 {
		v, err := url.PathUnescape(name[atIndex+1:])
		if err != nil {
			return PackageURL{}, fmt.Errorf("failed to unescape purl version: %s", err)
		}
		version = v

		unecapeName, err := url.PathUnescape(name[:atIndex])
		if err != nil {
			return PackageURL{}, fmt.Errorf("failed to unescape purl name: %s", err)
		}
		name = unecapeName
	}
	var namespaces []string

	if index != -1 {
		remainder = remainder[:index]

		for _, item := range strings.Split(remainder, "/") {
			if item != "" {
				unescaped, err := url.PathUnescape(item)
				if err != nil {
					return PackageURL{}, fmt.Errorf("failed to unescape path: %s", err)
				}
				namespaces = append(namespaces, unescaped)
			}
		}
	}
	namespace := strings.Join(namespaces, "/")
	namespace = typeAdjustNamespace(purlType, namespace)

	// Fail if name is empty at this point
	if name == "" {
		return PackageURL{}, errors.New("name is required")
	}

	err := validCustomRules(purlType, name, namespace, version, qualifiers)
	if err != nil {
		return PackageURL{}, err
	}

	return PackageURL{
		Type:       purlType,
		Namespace:  namespace,
		Name:       name,
		Version:    version,
		Qualifiers: qualifiers,
		Subpath:    substring,
	}, nil
}

// Make any purl type-specific adjustments to the parsed namespace.
// See https://github.com/package-url/purl-spec#known-purl-types
func typeAdjustNamespace(purlType, ns string) string {
	switch purlType {
	case TypeBitbucket, TypeDebian, TypeGithub, TypeGolang, TypeNPM, TypeRPM:
		return strings.ToLower(ns)
	}
	return ns
}

// Make any purl type-specific adjustments to the parsed name.
// See https://github.com/package-url/purl-spec#known-purl-types
func typeAdjustName(purlType, name string) string {
	switch purlType {
	case TypeBitbucket, TypeDebian, TypeGithub, TypeGolang, TypeNPM:
		return strings.ToLower(name)
	case TypePyPi:
		return strings.ToLower(strings.ReplaceAll(name, "_", "-"))
	}
	return name
}

// validQualifierKey validates a qualifierKey against our QualifierKeyPattern.
func validQualifierKey(key string) bool {
	return QualifierKeyPattern.MatchString(key)
}

// validCustomRules evaluates additional rules for each package url type, as specified in the package-url specification.
// On success, it returns nil. On failure, a descriptive error will be returned.
func validCustomRules(purlType, name, ns, version string, qualifiers Qualifiers) error {
	q := qualifiers.Map()
	switch purlType {
	case TypeConan:
		if ns != "" {
			if val, ok := q["channel"]; ok {
				if val == "" {
					return errors.New("the qualifier channel must be not empty if namespace is present")
				}
			} else {
				return errors.New("channel qualifier does not exist")
			}
		} else {
			if val, ok := q["channel"]; ok {
				if val != "" {
					return errors.New("namespace is required if channel is non empty")
				}
			}
		}
	case TypeSwift:
		if ns == "" {
			return errors.New("namespace is required")
		}
		if version == "" {
			return errors.New("version is required")
		}
	case TypeCran:
		if version == "" {
			return errors.New("version is required")
		}
	}
	return nil
}
