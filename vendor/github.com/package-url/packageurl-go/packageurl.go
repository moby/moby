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
	"slices"
	"strings"
)

var (
	// QualifierKeyPattern describes a valid qualifier key:
	//
	// - The key must be composed only of ASCII letters and numbers, '.',
	//   '-' and '_' (period, dash and underscore).
	// - A key cannot start with a number.
	QualifierKeyPattern = regexp.MustCompile(`^[A-Za-z\.\-_][0-9A-Za-z\.\-_]*$`)
	// TypePattern describes a valid type:
	//
	// - The type must be composed only of ASCII letters and numbers, '.',
	// '+' and '-' (period, plus and dash).
	// - A type cannot start with a number.
	TypePattern = regexp.MustCompile(`^[A-Za-z\.\-\+][0-9A-Za-z\.\-\+]*$`)
)

// These are the known purl types as defined in the spec. Some of these require
// special treatment during parsing.
// https://github.com/package-url/purl-spec#known-purl-types
var (
	// TypeAlpm is a pkg:alpm purl.
	TypeAlpm = "alpm"
	// TypeApk is a pkg:apk purl.
	TypeApk = "apk"
	// TypeBitbucket is a pkg:bitbucket purl.
	TypeBitbucket = "bitbucket"
	// TypeBitnami is a pkg:bitnami purl.
	TypeBitnami = "bitnami"
	// TypeCargo is a pkg:cargo purl.
	TypeCargo = "cargo"
	// TypeChromeExtension is a pkg:chrome-extension purl.
	TypeChromeExtension = "chrome-extension"
	// TypeCocoapods is a pkg:cocoapods purl.
	TypeCocoapods = "cocoapods"
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
	// TypeHuggingface is pkg:huggingface purl.
	TypeHuggingface = "huggingface"
	// TypeMLflow is pkg:mlflow purl.
	TypeMLFlow = "mlflow"
	// TypeMaven is a pkg:maven purl.
	TypeMaven = "maven"
	// TypeNPM is a pkg:npm purl.
	TypeNPM = "npm"
	// TypeNuget is a pkg:nuget purl.
	TypeNuget = "nuget"
	// TypeOCI is a pkg:oci purl.
	TypeOCI = "oci"
	// TypeOTP is a pkg:otp purl.
	TypeOTP = "otp"
	// TypePub is a pkg:pub purl.
	TypePub = "pub"
	// TypePyPi is a pkg:pypi purl.
	TypePyPi = "pypi"
	// TypeQPKG is a pkg:qpkg purl.
	TypeQpkg = "qpkg"
	// TypeRPM is a pkg:rpm purl.
	TypeRPM = "rpm"
	// TypeSWID is a pkg:swid purl.
	TypeSWID = "swid"
	// TypeSwift is a pkg:swift purl.
	TypeSwift = "swift"
	// TypeVcpkg is a pkg:vcpkg purl.
	TypeVcpkg = "vcpkg"
	// TypeVSCodeExtension is a pkg:vscode-extension purl.
	TypeVSCodeExtension = "vscode-extension"
	// TypeYocto is a pkg:yocto purl.
	TypeYocto = "yocto"

	// KnownTypes is a map of types that are officially supported by the spec.
	// See https://github.com/package-url/purl-spec/blob/master/PURL-TYPES.rst#known-purl-types
	KnownTypes = map[string]struct{}{
		TypeAlpm:            {},
		TypeApk:             {},
		TypeBitbucket:       {},
		TypeBitnami:         {},
		TypeCargo:           {},
		TypeChromeExtension: {},
		TypeCocoapods:       {},
		TypeComposer:        {},
		TypeConan:           {},
		TypeConda:           {},
		TypeCpan:            {},
		TypeCran:            {},
		TypeDebian:          {},
		TypeDocker:          {},
		TypeGem:             {},
		TypeGeneric:         {},
		TypeGithub:          {},
		TypeGolang:          {},
		TypeHackage:         {},
		TypeHex:             {},
		TypeHuggingface:     {},
		TypeMaven:           {},
		TypeMLFlow:          {},
		TypeNPM:             {},
		TypeNuget:           {},
		TypeOCI:             {},
		TypeOTP:             {},
		TypePub:             {},
		TypePyPi:            {},
		TypeQpkg:            {},
		TypeRPM:             {},
		TypeSWID:            {},
		TypeSwift:           {},
		TypeVcpkg:           {},
		TypeVSCodeExtension: {},
		TypeYocto:           {},
	}

	TypeApache      = "apache"
	TypeAndroid     = "android"
	TypeAtom        = "atom"
	TypeBower       = "bower"
	TypeBrew        = "brew"
	TypeBuildroot   = "buildroot"
	TypeCarthage    = "carthage"
	TypeChef        = "chef"
	TypeChocolatey  = "chocolatey"
	TypeClojars     = "clojars"
	TypeCoreos      = "coreos"
	TypeCpan        = "cpan"
	TypeCtan        = "ctan"
	TypeCrystal     = "crystal"
	TypeDrupal      = "drupal"
	TypeDtype       = "dtype"
	TypeDub         = "dub"
	TypeElm         = "elm"
	TypeEclipse     = "eclipse"
	TypeGitea       = "gitea"
	TypeGitlab      = "gitlab"
	TypeGradle      = "gradle"
	TypeGuix        = "guix"
	TypeHaxe        = "haxe"
	TypeHelm        = "helm"
	TypeJulia       = "julia"
	TypeLua         = "lua"
	TypeMelpa       = "melpa"
	TypeMeteor      = "meteor"
	TypeNim         = "nim"
	TypeNix         = "nix"
	TypeOpam        = "opam"
	TypeOpenwrt     = "openwrt"
	TypeOsgi        = "osgi"
	TypeP2          = "p2"
	TypePear        = "pear"
	TypePecl        = "pecl"
	TypePERL6       = "perl6"
	TypePlatformio  = "platformio"
	TypeEbuild      = "ebuild"
	TypePuppet      = "puppet"
	TypeSourceforge = "sourceforge"
	TypeSublime     = "sublime"
	TypeTerraform   = "terraform"
	TypeVagrant     = "vagrant"
	TypeVim         = "vim"
	TypeWORDPRESS   = "wordpress"

	// CandidateTypes is a map of types that are not yet officially supported by the spec,
	// but are being considered for inclusion.
	// See https://github.com/package-url/purl-spec/blob/master/PURL-TYPES.rst#other-candidate-types-to-define
	CandidateTypes = map[string]struct{}{
		TypeApache:      {},
		TypeAndroid:     {},
		TypeAtom:        {},
		TypeBower:       {},
		TypeBrew:        {},
		TypeBuildroot:   {},
		TypeCarthage:    {},
		TypeChef:        {},
		TypeChocolatey:  {},
		TypeClojars:     {},
		TypeCoreos:      {},
		TypeCtan:        {},
		TypeCrystal:     {},
		TypeDrupal:      {},
		TypeDtype:       {},
		TypeDub:         {},
		TypeElm:         {},
		TypeEclipse:     {},
		TypeGitea:       {},
		TypeGitlab:      {},
		TypeGradle:      {},
		TypeGuix:        {},
		TypeHaxe:        {},
		TypeHelm:        {},
		TypeJulia:       {},
		TypeLua:         {},
		TypeMelpa:       {},
		TypeMeteor:      {},
		TypeNim:         {},
		TypeNix:         {},
		TypeOpam:        {},
		TypeOpenwrt:     {},
		TypeOsgi:        {},
		TypeP2:          {},
		TypePear:        {},
		TypePecl:        {},
		TypePERL6:       {},
		TypePlatformio:  {},
		TypeEbuild:      {},
		TypePuppet:      {},
		TypeSourceforge: {},
		TypeSublime:     {},
		TypeTerraform:   {},
		TypeVagrant:     {},
		TypeVim:         {},
		TypeWORDPRESS:   {},
		TypeYocto:       {},
	}
)

// Qualifier represents a single key=value qualifier in the package url
type Qualifier struct {
	Key   string
	Value string
}

// String returns a canonical string representation of the qualifier according to [SPEC].
//
// [SPEC] https://github.com/package-url/purl-spec/blob/main/PURL-SPECIFICATION.rst#rules-for-each-purl-component
func (q Qualifier) String() string {
	// A value must be a percent-encoded string
	var b strings.Builder
	escapeQualifier(&b, q.Key)
	b.WriteByte('=')
	escapeQualifier(&b, q.Value)
	return b.String()
}

// Qualifiers is a slice of key=value pairs, with order preserved as it appears
// in the package URL.
type Qualifiers []Qualifier

// String returns a canonical string representation of the qualifiers as keys + values.
// Canonical form requires qualifier keys to be lexicographically ordered.
// The leading `?` qualifier component delimiter is excluded.
func (q Qualifiers) String() string {
	if len(q) == 0 {
		return ""
	}
	slices.SortFunc(q, func(a, b Qualifier) int { return strings.Compare(a.Key, b.Key) })
	var b strings.Builder
	// Estimate capacity: each qualifier needs key + "=" + value + "&"
	b.Grow(len(q) * 32)
	for i, qq := range q {
		if i > 0 {
			b.WriteByte('&')
		}
		escapeQualifier(&b, qq.Key)
		b.WriteByte('=')
		escapeQualifier(&b, qq.Value)
	}
	return b.String()
}

// escapeQualifier escapes a qualifier key or value for use in the query string.
// Per purl spec, ':' is NOT encoded but most other special characters are.
func escapeQualifier(b *strings.Builder, s string) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isQualifierSafe(c) {
			b.WriteByte(c)
		} else {
			writePercentEncodedByte(b, c)
		}
	}
}

// isQualifierSafe reports whether c can appear unencoded in a purl qualifier.
// Per purl spec, ':' is allowed unencoded in qualifier values.
func isQualifierSafe(c byte) bool {
	// Standard unreserved characters plus ':'
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
		c == '-' || c == '.' || c == '_' || c == '~' || c == ':'
}

// QualifiersFromMap constructs a Qualifiers slice from a string map. To get a
// deterministic qualifier order (despite maps not providing any iteration order
// guarantees) the returned Qualifiers are sorted in increasing order of key.
func QualifiersFromMap(mm map[string]string) Qualifiers {
	q := make(Qualifiers, 0, len(mm))

	for k, v := range mm {
		q = append(q, Qualifier{Key: k, Value: v})
	}

	// sort for deterministic qualifier order
	slices.SortFunc(q, func(a, b Qualifier) int { return strings.Compare(a.Key, b.Key) })

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

func (qq *Qualifiers) Normalize() error {
	qs := *qq
	normedQQ := make(Qualifiers, 0, len(qs))
	for _, q := range qs {
		if q.Key == "" {
			return fmt.Errorf("key is missing from qualifier: %v", q)
		}
		if q.Value == "" {
			// Empty values are equivalent to the key being omitted from the PackageURL.
			continue
		}
		key := toLowerASCII(q.Key)
		if !validQualifierKey(key) {
			return fmt.Errorf("invalid qualifier key: %q", key)
		}
		normedQQ = append(normedQQ, Qualifier{key, q.Value})
	}
	slices.SortFunc(normedQQ, func(a, b Qualifier) int { return strings.Compare(a.Key, b.Key) })
	for i := 1; i < len(normedQQ); i++ {
		if normedQQ[i-1].Key == normedQQ[i].Key {
			return fmt.Errorf("duplicate qualifier key: %q", normedQQ[i].Key)
		}
	}
	*qq = normedQQ
	return nil
}

// toLowerASCII returns s with all ASCII uppercase letters converted to lowercase.
// It avoids allocation if s is already lowercase.
func toLowerASCII(s string) string {
	needsConvert := -1
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'A' && c <= 'Z' {
			needsConvert = i
			break
		}
	}
	if needsConvert < 0 {
		return s
	}

	b := make([]byte, len(s))
	copy(b, s[:needsConvert])
	for i := needsConvert; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
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

// ToString returns a canonical string representation of the qualifier according to [SPEC].
//
// [SPEC] https://github.com/package-url/purl-spec/blob/main/PURL-SPECIFICATION.rst#rules-for-each-purl-component
func (p *PackageURL) ToString() string {
	var b strings.Builder
	// Estimate capacity for typical purl, including component delimiters.
	b.Grow(4 + len(p.Type) + 1 + len(p.Namespace) + 1 + len(p.Name) + 1 + len(p.Version) + len(p.Subpath) + 32)

	b.WriteString("pkg:")
	b.WriteString(p.Type)

	// Each namespace segment shall be a percent-encoded string.
	if p.Namespace != "" {
		start := 0
		for i := 0; i <= len(p.Namespace); i++ {
			if i == len(p.Namespace) || p.Namespace[i] == '/' {
				if i > start {
					b.WriteByte('/')
					writePercentEncodedString(&b, p.Namespace[start:i])
				}
				start = i + 1
			}
		}
	}

	// A name shall be a percent-encoded string.
	b.WriteByte('/')
	writePercentEncodedString(&b, p.Name)

	if p.Version != "" {
		// A version shall be a percent-encoded string.
		b.WriteByte('@')
		writePercentEncodedString(&b, p.Version)
	}

	if len(p.Qualifiers) > 0 {
		b.WriteByte('?')
		b.WriteString(p.Qualifiers.String())
	}

	// Each subpath segment shall be a percent-encoded string.
	if p.Subpath != "" {
		b.WriteByte('#')
		escapeSubpath(&b, p.Subpath)
	}

	return b.String()
}

func (p PackageURL) String() string {
	return p.ToString()
}

// FromString parses a valid package url string into a [PackageURL].
func FromString(purl string) (PackageURL, error) {
	// Check scheme
	if len(purl) < 4 || toLowerASCII(purl[:4]) != "pkg:" {
		return PackageURL{}, fmt.Errorf("purl scheme is not \"pkg\": %q", purl)
	}

	remainder := purl[4:]

	// Handle pkg:/ and pkg:// formats by stripping leading slashes
	for len(remainder) > 0 && remainder[0] == '/' {
		remainder = remainder[1:]
	}

	// Extract fragment (subpath)
	var subpath string
	if idx := strings.IndexByte(remainder, '#'); idx != -1 {
		// A subpath is a percent-encoded string and must be decoded like the
		// other components (namespace, name, version).
		decoded, err := percentDecodeSubpath(remainder[idx+1:])
		if err != nil {
			return PackageURL{}, fmt.Errorf("error unescaping subpath: %w", err)
		}
		subpath = decoded
		remainder = remainder[:idx]
	}

	// Extract query string (qualifiers)
	var rawQuery string
	if idx := strings.IndexByte(remainder, '?'); idx != -1 {
		rawQuery = remainder[idx+1:]
		remainder = remainder[:idx]
	}

	// Extract type
	typ, remainder, ok := strings.Cut(remainder, "/")
	if !ok {
		return PackageURL{}, fmt.Errorf("purl is missing type or name")
	}
	typ = toLowerASCII(typ)

	// Parse qualifiers
	qualifiers, err := parseQualifiers(rawQuery)
	if err != nil {
		return PackageURL{}, fmt.Errorf("invalid qualifiers: %w", err)
	}

	// Parse namespace, name, version
	namespace, name, version, err := separateNamespaceNameVersion(typ, remainder)
	if err != nil {
		return PackageURL{}, err
	}

	pURL := PackageURL{
		Qualifiers: qualifiers,
		Type:       typ,
		Namespace:  namespace,
		Name:       name,
		Version:    version,
		Subpath:    subpath,
	}

	err = pURL.Normalize()
	return pURL, err
}

// Normalize converts p to its canonical form, returning an error if p is invalid.
func (p *PackageURL) Normalize() error {
	typ := strings.ToLower(p.Type)
	if !validType(typ) {
		return fmt.Errorf("invalid type %q", typ)
	}
	namespace := strings.Trim(p.Namespace, "/")
	if err := p.Qualifiers.Normalize(); err != nil {
		return fmt.Errorf("invalid qualifiers: %v", err)
	}
	if p.Name == "" {
		return errors.New("purl is missing name")
	}
	subpath := strings.Trim(p.Subpath, "/")
	segs := strings.Split(p.Subpath, "/")
	for i, s := range segs {
		if (s == "." || s == "..") && i != 0 {
			return fmt.Errorf("invalid Package URL subpath: %q", p.Subpath)
		}
	}
	*p = PackageURL{
		Type:       typ,
		Namespace:  typeAdjustNamespace(typ, namespace),
		Name:       typeAdjustName(typ, p.Name, p.Qualifiers),
		Version:    typeAdjustVersion(typ, p.Version),
		Qualifiers: p.Qualifiers,
		Subpath:    subpath,
	}
	return validCustomRules(*p)
}

// percentDecode percent-decodes a purl component according to [Encoding].
//
// [Encoding] https://github.com/package-url/purl-spec/blob/main/PURL-SPECIFICATION.rst#character-encoding
func percentDecode(s string) (string, error) {
	// Note: uses [url.PathUnescape] instead of [url.QueryUnescape] to treat '+' characters
	// literally (not as space).
	return url.PathUnescape(s)
}

// percentDecodeSubpath percent-decodes a subpath by decoding each '/'-separated
// segment on its own, so an encoded slash inside a segment is not treated as a
// segment separator. This mirrors the per-segment decoding done for the namespace.
func percentDecodeSubpath(s string) (string, error) {
	if !strings.Contains(s, "%") {
		return s, nil
	}
	segments := strings.Split(s, "/")
	for i, segment := range segments {
		decoded, err := percentDecode(segment)
		if err != nil {
			return "", err
		}
		segments[i] = decoded
	}
	return strings.Join(segments, "/"), nil
}

// writePercentEncodedString percent-encodes s as a purl path segment and writes it to the builder.
func writePercentEncodedString(b *strings.Builder, s string) {
	// Check if we need to escape at all
	needsEscape := false
	for i := 0; i < len(s); i++ {
		if !isPathSegmentSafe(s[i]) {
			needsEscape = true
			break
		}
	}
	if !needsEscape {
		b.WriteString(s)
		return
	}

	// Need to escape - process character by character
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isPathSegmentSafe(c) {
			b.WriteByte(c)
		} else {
			writePercentEncodedByte(b, c)
		}
	}
}

// writePercentEncodedByte percent-encodes the given byte as per [RFC-3986] and writes the result to
// the supplied [strings.Builder].
//
// [RFC-3986]: https://datatracker.ietf.org/doc/html/rfc3986#page-12
func writePercentEncodedByte(b *strings.Builder, c byte) {
	b.WriteByte('%')
	b.WriteByte(hexUpper[c>>4])
	b.WriteByte(hexUpper[c&0x0f])
}

// isPathSegmentSafe reports whether c can appear unencoded in a purl path segment.
// Per the purl spec, the only characters that must NOT be percent-encoded are:
// alphanumerics, the unreserved punctuation '-', '.', '_', '~', and ':'.
// All other characters — including RFC 3986 sub-delimiters such as '(', ')', '!',
// '$', '&', "'", '*', ',', ';', '=' — must be percent-encoded.
//
// See https://ecma-tc54.github.io/ECMA-427/#sec-purl-specification-character-encoding
func isPathSegmentSafe(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
		c == '-' || c == '.' || c == '_' || c == '~' || c == ':'
}

// escapeSubpath escapes a subpath, handling segments separated by '/'.
func escapeSubpath(b *strings.Builder, s string) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '/' {
			b.WriteByte('/')
		} else if isPathSegmentSafe(c) {
			b.WriteByte(c)
		} else {
			writePercentEncodedByte(b, c)
		}
	}
}

const hexUpper = "0123456789ABCDEF"

// separateNamespaceNameVersion parses the <namespace>/<name>@<version> part of a purl (the
// remainder parameter) into its constituent components. It aims to follow the [HOW-TO-PARSE]
// procedure.
//
// [HOW-TO-PARSE]: https://github.com/package-url/purl-spec/blob/main/docs/how-to-parse.md
func separateNamespaceNameVersion(purlType string, remainder string) (ns, name, version string, err error) {
	// NPM purls can have a namespace ("scope") that starts with an '@' character.
	// For example, "pkg:npm/@babel/core".
	// For any other purl type this indicates malformed purl input.
	if purlType != TypeNPM && strings.HasPrefix(remainder, "@") {
		return "", "", "", fmt.Errorf("purl is missing name")
	}
	// A leading '@' is only valid for npm when it introduces a scope, which
	// requires a '/' to separate the scope from the name (for example,
	// "pkg:npm/@babel/core"). A remainder like "@4.17.21" has no '/', so it is a
	// bare scope with no name and must be rejected the same way as v0.1.3 did.
	if purlType == TypeNPM && strings.HasPrefix(remainder, "@") && !strings.Contains(remainder, "/") {
		return "", "", "", fmt.Errorf("purl is missing name")
	}

	// Split the remainder once from right on '@'.
	// The left side is the remainder.
	if strings.LastIndex(remainder, "@") > 0 {
		remainder, version = rightmostSplit(remainder, "@")
		// Percent-decode the right side. This is the version.
		version, err = percentDecode(version)
		if err != nil {
			return "", "", "", fmt.Errorf("error unescaping version: %w", err)
		}
	}

	// Split this once from right on '/'.
	// The left side is the remainder.
	remainder, name = rightmostSplit(remainder, "/")
	// Percent-decode the right side. This is the name.
	name, err = percentDecode(name)
	if err != nil {
		return "", "", "", fmt.Errorf("error unescaping name: %w", err)
	}

	// Split the remainder on '/'.
	segments := strings.Split(remainder, "/")
	nsSegments := []string{}
	for _, segment := range segments {
		// Discard any empty segment from that split.
		if segment == "" {
			continue
		}
		// Percent-decode each segment.
		nsSegment, err := percentDecode(segment)
		if err != nil {
			return "", "", "", fmt.Errorf("error unescaping namespace: %w", err)
		}
		nsSegments = append(nsSegments, nsSegment)
	}
	// Join segments back with a '/'.
	ns = strings.Join(nsSegments, "/")

	if name == "" {
		return "", "", "", fmt.Errorf("purl is missing name")
	}

	return ns, name, version, nil
}

// rightmostSplit splits the input path on a given delimiter such that the lhs returns the string to
// the left of the right-most delimiter and rhs return the string to the right of the right-most
// delimiter. For example, given path "github.com/package-url/packageurl-go" and delimiter "/" the
// lhs will be "github.com/package-url" and rhs will be "packageurl-go".
func rightmostSplit(path string, delim string) (lhs, rhs string) {
	lastSepIdx := strings.LastIndex(path, delim)
	rhs = path[lastSepIdx+1:]
	if lastSepIdx >= 0 {
		lhs = path[:lastSepIdx]
	}
	return lhs, rhs
}

func parseQualifiers(rawQuery string) (Qualifiers, error) {
	// we need to parse the qualifiers ourselves and cannot rely on the `url.Query` type because
	// that uses a map, meaning it's unordered. We want to keep the order of the qualifiers, so this
	// function re-implements the `url.parseQuery` function based on our `Qualifier` type. Most of
	// the code here is taken from `url.parseQuery`.
	q := Qualifiers{}
	for rawQuery != "" {
		var key string
		key, rawQuery, _ = strings.Cut(rawQuery, "&")
		if strings.Contains(key, ";") {
			return nil, fmt.Errorf("invalid semicolon separator in query")
		}
		if key == "" {
			continue
		}
		// The key is the lowercase left side.
		key, value, _ := strings.Cut(key, "=")
		key = strings.ToLower(key)

		if !validQualifierKey(key) {
			return nil, fmt.Errorf("invalid qualifier key: '%s'", key)
		}

		// The value is the percent-decoded right side.
		value, err := percentDecode(value)
		if err != nil {
			return nil, fmt.Errorf("error unescaping qualifier value %q", value)
		}

		q = append(q, Qualifier{Key: key, Value: value})
	}
	return q, nil
}

// Make any purl type-specific adjustments to the parsed namespace.
// See https://github.com/package-url/purl-spec#known-purl-types
func typeAdjustNamespace(purlType, ns string) string {
	switch purlType {
	case TypeAlpm,
		TypeApk,
		TypeBitbucket,
		TypeBrew,
		TypeComposer,
		TypeDebian,
		TypeGithub,
		TypeGolang,
		TypeRPM,
		TypeQpkg:
		return strings.ToLower(ns)
	}
	return ns
}

// Make any purl type-specific adjustments to the parsed name.
// See https://github.com/package-url/purl-spec#known-purl-types
func typeAdjustName(purlType, name string, qualifiers Qualifiers) string {
	quals := qualifiers.Map()
	switch purlType {
	case TypeAlpm,
		TypeApk,
		TypeBitbucket,
		TypeBitnami,
		TypeBrew,
		TypeChromeExtension,
		TypeComposer,
		TypeDebian,
		TypeGithub,
		TypeGolang:
		return strings.ToLower(name)
	case TypePyPi:
		return strings.ToLower(strings.ReplaceAll(name, "_", "-"))
	case TypeMLFlow:
		return adjustMlflowName(name, quals)
	}
	return name
}

// Make any purl type-specific adjustments to the parsed version.
// See https://github.com/package-url/purl-spec#known-purl-types
func typeAdjustVersion(purlType, version string) string {
	switch purlType {
	case TypeHuggingface:
		return strings.ToLower(version)
	}
	return version
}

// https://github.com/package-url/purl-spec/blob/master/PURL-TYPES.rst#mlflow
func adjustMlflowName(name string, qualifiers map[string]string) string {
	if repo, ok := qualifiers["repository_url"]; ok {
		if strings.Contains(repo, "databricks") {
			// Databricks is case-insensitive and must be lowercased
			return strings.ToLower(name)
		}

		// Azure ML is case-sensitive and must be kept as-is
		// Unknown repository type, keep as-is
		return name

	} else {
		// No repository qualifier given, keep as-is
		return name
	}
}

// validQualifierKey validates a qualifierKey against our QualifierKeyPattern.
// The key must be composed only of ASCII letters and numbers, '.', '-' and '_'.
// A key cannot start with a number.
// See https://ecma-tc54.github.io/ECMA-427/#sec-purl-specification-rules-qualifiers.
func validQualifierKey(key string) bool {
	if len(key) == 0 {
		return false
	}
	// First character: must be a-z, A-Z, '.', '-', or '_'
	c := key[0]
	if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '.' || c == '-' || c == '_') {
		return false
	}
	// Remaining characters: a-z, A-Z, 0-9, '.', '-', or '_'
	for i := 1; i < len(key); i++ {
		c = key[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// validType validates a type against our TypePattern.
// The type must be composed only of ASCII letters and numbers, '.', '+' and '-'.
// A type cannot start with a number.
// See https://ecma-tc54.github.io/ECMA-427/#sec-purl-specification-rules-type.
func validType(typ string) bool {
	if len(typ) == 0 {
		return false
	}
	// First character: must be a-z, A-Z, '.', '-', or '+'
	c := typ[0]
	if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '.' || c == '-' || c == '+') {
		return false
	}
	// Remaining characters: a-z, A-Z, 0-9, '.', '-', or '+'
	for i := 1; i < len(typ); i++ {
		c = typ[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '+') {
			return false
		}
	}
	return true
}

// validChromeExtensionName checks the name against ^[a-z]{32}$.
func validChromeExtensionName(name string) bool {
	if len(name) != 32 {
		return false
	}
	for i := 0; i < len(name); i++ {
		if c := name[i]; c < 'a' || c > 'z' {
			return false
		}
	}
	return true
}

// validChromeExtensionVersion checks the version against ^\d+(\.\d+){0,3}$.
func validChromeExtensionVersion(version string) bool {
	segments := 1
	digitsInSegment := 0
	for i := 0; i < len(version); i++ {
		c := version[i]
		if c >= '0' && c <= '9' {
			digitsInSegment++
			continue
		}
		if c == '.' {
			if digitsInSegment == 0 {
				return false
			}
			segments++
			digitsInSegment = 0
			continue
		}
		return false
	}
	return digitsInSegment > 0 && segments <= 4
}

// validCustomRules evaluates additional rules for each package url type, as specified in the package-url specification.
// On success, it returns nil. On failure, a descriptive error will be returned.
func validCustomRules(p PackageURL) error {
	switch p.Type {
	case TypeChromeExtension:
		if p.Namespace != "" {
			return errors.New("a chrome-extension purl must not have a namespace")
		}
		if !validChromeExtensionName(p.Name) {
			return errors.New("a chrome-extension name must be 32 lowercase ASCII letters")
		}
		if p.Version != "" && !validChromeExtensionVersion(p.Version) {
			return errors.New("a chrome-extension version must be 1 to 4 dot-separated integers")
		}
	case TypeCpan:
		// It MUST be written uppercase.
		if strings.ToUpper(p.Namespace) != p.Namespace {
			return errors.New("a cpan purl namespace must use uppercase characters")
		}

		// A distribution name MUST NOT contain the string '::'.
		distName := p.Name
		if strings.Contains(distName, "::") {
			return errors.New("a cpan distribution name must not contain '::'")
		}
	case TypeJulia:
		// The spec prohibits a namespace.
		if p.Namespace != "" {
			return errors.New("a julia purl must not have a namespace")
		}
		// The spec requires the presence of a uuid qualifier.
		if _, ok := p.Qualifiers.Map()["uuid"]; !ok {
			return errors.New("a julia purl must have a uuid qualifier")
		}
	case TypeOTP:
		// The spec prohibits a namespace.
		if p.Namespace != "" {
			return errors.New("an otp purl must not have a namespace")
		}
	case TypeSwift:
		if p.Namespace == "" {
			return errors.New("namespace is required")
		}
	case TypeVcpkg:
		if p.Namespace != "" {
			return errors.New("a vcpkg purl must not have a namespace")
		}
	case TypeVSCodeExtension:
		if p.Namespace == "" {
			return errors.New("namespace is required")
		}
	}
	return nil
}
