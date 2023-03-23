/*
   Copyright The containerd Authors.

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

// Package docker provides a general type to represent any way of referencing images within the registry.
// Its main purpose is to abstract tags and digests (content-addressable hash).
//
// Grammar
//
//	reference                       := name [ ":" tag ] [ "@" digest ]
//	name                            := [domain '/'] path-component ['/' path-component]*
//	domain                          := domain-component ['.' domain-component]* [':' port-number]
//	domain-component                := /([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9])/
//	port-number                     := /[0-9]+/
//	path-component                  := alpha-numeric [separator alpha-numeric]*
//	alpha-numeric                   := /[a-z0-9]+/
//	separator                       := /[_.]|__|[-]*/
//
//	tag                             := /[\w][\w.-]{0,127}/
//
//	digest                          := digest-algorithm ":" digest-hex
//	digest-algorithm                := digest-algorithm-component [ digest-algorithm-separator digest-algorithm-component ]*
//	digest-algorithm-separator      := /[+.-_]/
//	digest-algorithm-component      := /[A-Za-z][A-Za-z0-9]*/
//	digest-hex                      := /[0-9a-fA-F]{32,}/ ; At least 128 bit digest value
//
//	identifier                      := /[a-f0-9]{64}/
//	short-identifier                := /[a-f0-9]{6,64}/
package docker

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/opencontainers/go-digest"
)

const (
	// NameTotalLengthMax is the maximum total number of characters in a repository name.
	NameTotalLengthMax = 255
)

var (
	// ErrReferenceInvalidFormat represents an error while trying to parse a string as a reference.
	ErrReferenceInvalidFormat = errors.New("invalid reference format")

	// ErrTagInvalidFormat represents an error while trying to parse a string as a tag.
	ErrTagInvalidFormat = errors.New("invalid tag format")

	// ErrDigestInvalidFormat represents an error while trying to parse a string as a tag.
	ErrDigestInvalidFormat = errors.New("invalid digest format")

	// ErrNameContainsUppercase is returned for invalid repository names that contain uppercase characters.
	ErrNameContainsUppercase = errors.New("repository name must be lowercase")

	// ErrNameEmpty is returned for empty, invalid repository names.
	ErrNameEmpty = errors.New("repository name must have at least one component")

	// ErrNameTooLong is returned when a repository name is longer than NameTotalLengthMax.
	ErrNameTooLong = fmt.Errorf("repository name must not be more than %v characters", NameTotalLengthMax)

	// ErrNameNotCanonical is returned when a name is not canonical.
	ErrNameNotCanonical = errors.New("repository name must be canonical")
)

// Reference is an opaque object reference identifier that may include
// modifiers such as a hostname, name, tag, and digest.
type Reference interface {
	// String returns the full reference
	String() string
}

// Field provides a wrapper type for resolving correct reference types when
// working with encoding.
type Field struct {
	reference Reference
}

// AsField wraps a reference in a Field for encoding.
func AsField(reference Reference) Field {
	return Field{reference}
}

// Reference unwraps the reference type from the field to
// return the Reference object. This object should be
// of the appropriate type to further check for different
// reference types.
func (f Field) Reference() Reference {
	return f.reference
}

// MarshalText serializes the field to byte text which
// is the string of the reference.
func (f Field) MarshalText() (p []byte, err error) {
	return []byte(f.reference.String()), nil
}

// UnmarshalText parses text bytes by invoking the
// reference parser to ensure the appropriately
// typed reference object is wrapped by field.
func (f *Field) UnmarshalText(p []byte) error {
	r, err := Parse(string(p))
	if err != nil {
		return err
	}

	f.reference = r
	return nil
}

// Named is an object with a full name
type Named interface {
	Reference
	Name() string
}

// Tagged is an object which has a tag
type Tagged interface {
	Reference
	Tag() string
}

// NamedTagged is an object including a name and tag.
type NamedTagged interface {
	Named
	Tag() string
}

// Digested is an object which has a digest
// in which it can be referenced by
type Digested interface {
	Reference
	Digest() digest.Digest
}

// Canonical reference is an object with a fully unique
// name including a name with domain and digest
type Canonical interface {
	Named
	Digest() digest.Digest
}

// namedRepository is a reference to a repository with a name.
// A namedRepository has both domain and path components.
type namedRepository interface {
	Named
	Domain() string
	Path() string
}

// Domain returns the domain part of the Named reference
func Domain(named Named) string {
	if r, ok := named.(namedRepository); ok {
		return r.Domain()
	}
	domain, _ := splitDomain(named.Name())
	return domain
}

// Path returns the name without the domain part of the Named reference
func Path(named Named) (name string) {
	if r, ok := named.(namedRepository); ok {
		return r.Path()
	}
	_, path := splitDomain(named.Name())
	return path
}

func splitDomain(name string) (string, string) {
	match := anchoredNameRegexp.FindStringSubmatch(name)
	if len(match) != 3 {
		return "", name
	}
	return match[1], match[2]
}

// SplitHostname splits a named reference into a
// hostname and name string. If no valid hostname is
// found, the hostname is empty and the full value
// is returned as name
// DEPRECATED: Use Domain or Path
func SplitHostname(named Named) (string, string) {
	if r, ok := named.(namedRepository); ok {
		return r.Domain(), r.Path()
	}
	return splitDomain(named.Name())
}

// Parse parses s and returns a syntactically valid Reference.
// If an error was encountered it is returned, along with a nil Reference.
// NOTE: Parse will not handle short digests.
func Parse(s string) (Reference, error) {
	matches := ReferenceRegexp.FindStringSubmatch(s)
	if matches == nil {
		if s == "" {
			return nil, ErrNameEmpty
		}
		if ReferenceRegexp.FindStringSubmatch(strings.ToLower(s)) != nil {
			return nil, ErrNameContainsUppercase
		}
		return nil, ErrReferenceInvalidFormat
	}

	if len(matches[1]) > NameTotalLengthMax {
		return nil, ErrNameTooLong
	}

	var repo repository

	nameMatch := anchoredNameRegexp.FindStringSubmatch(matches[1])
	if len(nameMatch) == 3 {
		repo.domain = nameMatch[1]
		repo.path = nameMatch[2]
	} else {
		repo.domain = ""
		repo.path = matches[1]
	}

	ref := reference{
		namedRepository: repo,
		tag:             matches[2],
	}
	if matches[3] != "" {
		var err error
		ref.digest, err = digest.Parse(matches[3])
		if err != nil {
			return nil, err
		}
	}

	r := getBestReferenceType(ref)
	if r == nil {
		return nil, ErrNameEmpty
	}

	return r, nil
}

// ParseNamed parses s and returns a syntactically valid reference implementing
// the Named interface. The reference must have a name and be in the canonical
// form, otherwise an error is returned.
// If an error was encountered it is returned, along with a nil Reference.
// NOTE: ParseNamed will not handle short digests.
func ParseNamed(s string) (Named, error) {
	named, err := ParseNormalizedNamed(s)
	if err != nil {
		return nil, err
	}
	if named.String() != s {
		return nil, ErrNameNotCanonical
	}
	return named, nil
}

// WithName returns a named object representing the given string. If the input
// is invalid ErrReferenceInvalidFormat will be returned.
func WithName(name string) (Named, error) {
	if len(name) > NameTotalLengthMax {
		return nil, ErrNameTooLong
	}

	match := anchoredNameRegexp.FindStringSubmatch(name)
	if match == nil || len(match) != 3 {
		return nil, ErrReferenceInvalidFormat
	}
	return repository{
		domain: match[1],
		path:   match[2],
	}, nil
}

// WithTag combines the name from "name" and the tag from "tag" to form a
// reference incorporating both the name and the tag.
func WithTag(name Named, tag string) (NamedTagged, error) {
	if !anchoredTagRegexp.MatchString(tag) {
		return nil, ErrTagInvalidFormat
	}
	var repo repository
	if r, ok := name.(namedRepository); ok {
		repo.domain = r.Domain()
		repo.path = r.Path()
	} else {
		repo.path = name.Name()
	}
	if canonical, ok := name.(Canonical); ok {
		return reference{
			namedRepository: repo,
			tag:             tag,
			digest:          canonical.Digest(),
		}, nil
	}
	return taggedReference{
		namedRepository: repo,
		tag:             tag,
	}, nil
}

// WithDigest combines the name from "name" and the digest from "digest" to form
// a reference incorporating both the name and the digest.
func WithDigest(name Named, digest digest.Digest) (Canonical, error) {
	if !anchoredDigestRegexp.MatchString(digest.String()) {
		return nil, ErrDigestInvalidFormat
	}
	var repo repository
	if r, ok := name.(namedRepository); ok {
		repo.domain = r.Domain()
		repo.path = r.Path()
	} else {
		repo.path = name.Name()
	}
	if tagged, ok := name.(Tagged); ok {
		return reference{
			namedRepository: repo,
			tag:             tagged.Tag(),
			digest:          digest,
		}, nil
	}
	return canonicalReference{
		namedRepository: repo,
		digest:          digest,
	}, nil
}

// TrimNamed removes any tag or digest from the named reference.
func TrimNamed(ref Named) Named {
	repo := repository{}
	if r, ok := ref.(namedRepository); ok {
		repo.domain, repo.path = r.Domain(), r.Path()
	} else {
		repo.domain, repo.path = splitDomain(ref.Name())
	}
	return repo
}

func getBestReferenceType(ref reference) Reference {
	if ref.Name() == "" {
		// Allow digest only references
		if ref.digest != "" {
			return digestReference(ref.digest)
		}
		return nil
	}
	if ref.tag == "" {
		if ref.digest != "" {
			return canonicalReference{
				namedRepository: ref.namedRepository,
				digest:          ref.digest,
			}
		}
		return ref.namedRepository
	}
	if ref.digest == "" {
		return taggedReference{
			namedRepository: ref.namedRepository,
			tag:             ref.tag,
		}
	}

	return ref
}

type reference struct {
	namedRepository
	tag    string
	digest digest.Digest
}

func (r reference) String() string {
	return r.Name() + ":" + r.tag + "@" + r.digest.String()
}

func (r reference) Tag() string {
	return r.tag
}

func (r reference) Digest() digest.Digest {
	return r.digest
}

type repository struct {
	domain string
	path   string
}

func (r repository) String() string {
	return r.Name()
}

func (r repository) Name() string {
	if r.domain == "" {
		return r.path
	}
	return r.domain + "/" + r.path
}

func (r repository) Domain() string {
	return r.domain
}

func (r repository) Path() string {
	return r.path
}

type digestReference digest.Digest

func (d digestReference) String() string {
	return digest.Digest(d).String()
}

func (d digestReference) Digest() digest.Digest {
	return digest.Digest(d)
}

type taggedReference struct {
	namedRepository
	tag string
}

func (t taggedReference) String() string {
	return t.Name() + ":" + t.tag
}

func (t taggedReference) Tag() string {
	return t.tag
}

type canonicalReference struct {
	namedRepository
	digest digest.Digest
}

func (c canonicalReference) String() string {
	return c.Name() + "@" + c.digest.String()
}

func (c canonicalReference) Digest() digest.Digest {
	return c.digest
}

var (
	// alphaNumericRegexp defines the alpha numeric atom, typically a
	// component of names. This only allows lower case characters and digits.
	alphaNumericRegexp = match(`[a-z0-9]+`)

	// separatorRegexp defines the separators allowed to be embedded in name
	// components. This allow one period, one or two underscore and multiple
	// dashes.
	separatorRegexp = match(`(?:[._]|__|[-]*)`)

	// nameComponentRegexp restricts registry path component names to start
	// with at least one letter or number, with following parts able to be
	// separated by one period, one or two underscore and multiple dashes.
	nameComponentRegexp = expression(
		alphaNumericRegexp,
		optional(repeated(separatorRegexp, alphaNumericRegexp)))

	// domainComponentRegexp restricts the registry domain component of a
	// repository name to start with a component as defined by DomainRegexp
	// and followed by an optional port.
	domainComponentRegexp = match(`(?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9])`)

	// DomainRegexp defines the structure of potential domain components
	// that may be part of image names. This is purposely a subset of what is
	// allowed by DNS to ensure backwards compatibility with Docker image
	// names.
	DomainRegexp = expression(
		domainComponentRegexp,
		optional(repeated(literal(`.`), domainComponentRegexp)),
		optional(literal(`:`), match(`[0-9]+`)))

	// TagRegexp matches valid tag names. From docker/docker:graph/tags.go.
	TagRegexp = match(`[\w][\w.-]{0,127}`)

	// anchoredTagRegexp matches valid tag names, anchored at the start and
	// end of the matched string.
	anchoredTagRegexp = anchored(TagRegexp)

	// DigestRegexp matches valid digests.
	DigestRegexp = match(`[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*[:][[:xdigit:]]{32,}`)

	// anchoredDigestRegexp matches valid digests, anchored at the start and
	// end of the matched string.
	anchoredDigestRegexp = anchored(DigestRegexp)

	// NameRegexp is the format for the name component of references. The
	// regexp has capturing groups for the domain and name part omitting
	// the separating forward slash from either.
	NameRegexp = expression(
		optional(DomainRegexp, literal(`/`)),
		nameComponentRegexp,
		optional(repeated(literal(`/`), nameComponentRegexp)))

	// anchoredNameRegexp is used to parse a name value, capturing the
	// domain and trailing components.
	anchoredNameRegexp = anchored(
		optional(capture(DomainRegexp), literal(`/`)),
		capture(nameComponentRegexp,
			optional(repeated(literal(`/`), nameComponentRegexp))))

	// ReferenceRegexp is the full supported format of a reference. The regexp
	// is anchored and has capturing groups for name, tag, and digest
	// components.
	ReferenceRegexp = anchored(capture(NameRegexp),
		optional(literal(":"), capture(TagRegexp)),
		optional(literal("@"), capture(DigestRegexp)))

	// IdentifierRegexp is the format for string identifier used as a
	// content addressable identifier using sha256. These identifiers
	// are like digests without the algorithm, since sha256 is used.
	IdentifierRegexp = match(`([a-f0-9]{64})`)

	// ShortIdentifierRegexp is the format used to represent a prefix
	// of an identifier. A prefix may be used to match a sha256 identifier
	// within a list of trusted identifiers.
	ShortIdentifierRegexp = match(`([a-f0-9]{6,64})`)

	// anchoredIdentifierRegexp is used to check or match an
	// identifier value, anchored at start and end of string.
	anchoredIdentifierRegexp = anchored(IdentifierRegexp)
)

// match compiles the string to a regular expression.
var match = regexp.MustCompile

// literal compiles s into a literal regular expression, escaping any regexp
// reserved characters.
func literal(s string) *regexp.Regexp {
	re := match(regexp.QuoteMeta(s))

	if _, complete := re.LiteralPrefix(); !complete {
		panic("must be a literal")
	}

	return re
}

// expression defines a full expression, where each regular expression must
// follow the previous.
func expression(res ...*regexp.Regexp) *regexp.Regexp {
	var s string
	for _, re := range res {
		s += re.String()
	}

	return match(s)
}

// optional wraps the expression in a non-capturing group and makes the
// production optional.
func optional(res ...*regexp.Regexp) *regexp.Regexp {
	return match(group(expression(res...)).String() + `?`)
}

// repeated wraps the regexp in a non-capturing group to get one or more
// matches.
func repeated(res ...*regexp.Regexp) *regexp.Regexp {
	return match(group(expression(res...)).String() + `+`)
}

// group wraps the regexp in a non-capturing group.
func group(res ...*regexp.Regexp) *regexp.Regexp {
	return match(`(?:` + expression(res...).String() + `)`)
}

// capture wraps the expression in a capturing group.
func capture(res ...*regexp.Regexp) *regexp.Regexp {
	return match(`(` + expression(res...).String() + `)`)
}

// anchored anchors the regular expression by adding start and end delimiters.
func anchored(res ...*regexp.Regexp) *regexp.Regexp {
	return match(`^` + expression(res...).String() + `$`)
}

var (
	legacyDefaultDomain = "index.docker.io"
	defaultDomain       = "docker.io"
	officialRepoName    = "library"
	defaultTag          = "latest"
)

// normalizedNamed represents a name which has been
// normalized and has a familiar form. A familiar name
// is what is used in Docker UI. An example normalized
// name is "docker.io/library/ubuntu" and corresponding
// familiar name of "ubuntu".
type normalizedNamed interface {
	Named
	Familiar() Named
}

// ParseNormalizedNamed parses a string into a named reference
// transforming a familiar name from Docker UI to a fully
// qualified reference. If the value may be an identifier
// use ParseAnyReference.
func ParseNormalizedNamed(s string) (Named, error) {
	if ok := anchoredIdentifierRegexp.MatchString(s); ok {
		return nil, fmt.Errorf("invalid repository name (%s), cannot specify 64-byte hexadecimal strings", s)
	}
	domain, remainder := splitDockerDomain(s)
	var remoteName string
	if tagSep := strings.IndexRune(remainder, ':'); tagSep > -1 {
		remoteName = remainder[:tagSep]
	} else {
		remoteName = remainder
	}
	if strings.ToLower(remoteName) != remoteName {
		return nil, errors.New("invalid reference format: repository name must be lowercase")
	}

	ref, err := Parse(domain + "/" + remainder)
	if err != nil {
		return nil, err
	}
	named, isNamed := ref.(Named)
	if !isNamed {
		return nil, fmt.Errorf("reference %s has no name", ref.String())
	}
	return named, nil
}

// ParseDockerRef normalizes the image reference following the docker convention. This is added
// mainly for backward compatibility.
// The reference returned can only be either tagged or digested. For reference contains both tag
// and digest, the function returns digested reference, e.g. docker.io/library/busybox:latest@
// sha256:7cc4b5aefd1d0cadf8d97d4350462ba51c694ebca145b08d7d41b41acc8db5aa will be returned as
// docker.io/library/busybox@sha256:7cc4b5aefd1d0cadf8d97d4350462ba51c694ebca145b08d7d41b41acc8db5aa.
func ParseDockerRef(ref string) (Named, error) {
	named, err := ParseNormalizedNamed(ref)
	if err != nil {
		return nil, err
	}
	if _, ok := named.(NamedTagged); ok {
		if canonical, ok := named.(Canonical); ok {
			// The reference is both tagged and digested, only
			// return digested.
			newNamed, err := WithName(canonical.Name())
			if err != nil {
				return nil, err
			}
			newCanonical, err := WithDigest(newNamed, canonical.Digest())
			if err != nil {
				return nil, err
			}
			return newCanonical, nil
		}
	}
	return TagNameOnly(named), nil
}

// splitDockerDomain splits a repository name to domain and remotename string.
// If no valid domain is found, the default domain is used. Repository name
// needs to be already validated before.
func splitDockerDomain(name string) (domain, remainder string) {
	i := strings.IndexRune(name, '/')
	if i == -1 || (!strings.ContainsAny(name[:i], ".:") && name[:i] != "localhost") {
		domain, remainder = defaultDomain, name
	} else {
		domain, remainder = name[:i], name[i+1:]
	}
	if domain == legacyDefaultDomain {
		domain = defaultDomain
	}
	if domain == defaultDomain && !strings.ContainsRune(remainder, '/') {
		remainder = officialRepoName + "/" + remainder
	}
	return
}

// familiarizeName returns a shortened version of the name familiar
// to the Docker UI. Familiar names have the default domain
// "docker.io" and "library/" repository prefix removed.
// For example, "docker.io/library/redis" will have the familiar
// name "redis" and "docker.io/dmcgowan/myapp" will be "dmcgowan/myapp".
// Returns a familiarized named only reference.
func familiarizeName(named namedRepository) repository {
	repo := repository{
		domain: named.Domain(),
		path:   named.Path(),
	}

	if repo.domain == defaultDomain {
		repo.domain = ""
		// Handle official repositories which have the pattern "library/<official repo name>"
		if split := strings.Split(repo.path, "/"); len(split) == 2 && split[0] == officialRepoName {
			repo.path = split[1]
		}
	}
	return repo
}

func (r reference) Familiar() Named {
	return reference{
		namedRepository: familiarizeName(r.namedRepository),
		tag:             r.tag,
		digest:          r.digest,
	}
}

func (r repository) Familiar() Named {
	return familiarizeName(r)
}

func (t taggedReference) Familiar() Named {
	return taggedReference{
		namedRepository: familiarizeName(t.namedRepository),
		tag:             t.tag,
	}
}

func (c canonicalReference) Familiar() Named {
	return canonicalReference{
		namedRepository: familiarizeName(c.namedRepository),
		digest:          c.digest,
	}
}

// TagNameOnly adds the default tag "latest" to a reference if it only has
// a repo name.
func TagNameOnly(ref Named) Named {
	if IsNameOnly(ref) {
		namedTagged, err := WithTag(ref, defaultTag)
		if err != nil {
			// Default tag must be valid, to create a NamedTagged
			// type with non-validated input the WithTag function
			// should be used instead
			panic(err)
		}
		return namedTagged
	}
	return ref
}

// ParseAnyReference parses a reference string as a possible identifier,
// full digest, or familiar name.
func ParseAnyReference(ref string) (Reference, error) {
	if ok := anchoredIdentifierRegexp.MatchString(ref); ok {
		return digestReference("sha256:" + ref), nil
	}
	if dgst, err := digest.Parse(ref); err == nil {
		return digestReference(dgst), nil
	}

	return ParseNormalizedNamed(ref)
}

// IsNameOnly returns true if reference only contains a repo name.
func IsNameOnly(ref Named) bool {
	if _, ok := ref.(NamedTagged); ok {
		return false
	}
	if _, ok := ref.(Canonical); ok {
		return false
	}
	return true
}

// FamiliarName returns the familiar name string
// for the given named, familiarizing if needed.
func FamiliarName(ref Named) string {
	if nn, ok := ref.(normalizedNamed); ok {
		return nn.Familiar().Name()
	}
	return ref.Name()
}

// FamiliarString returns the familiar string representation
// for the given reference, familiarizing if needed.
func FamiliarString(ref Reference) string {
	if nn, ok := ref.(normalizedNamed); ok {
		return nn.Familiar().String()
	}
	return ref.String()
}

// FamiliarMatch reports whether ref matches the specified pattern.
// See https://godoc.org/path#Match for supported patterns.
func FamiliarMatch(pattern string, ref Reference) (bool, error) {
	matched, err := path.Match(pattern, FamiliarString(ref))
	if namedRef, isNamed := ref.(Named); isNamed && !matched {
		matched, _ = path.Match(pattern, FamiliarName(namedRef))
	}
	return matched, err
}
