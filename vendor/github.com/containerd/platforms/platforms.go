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

// Package platforms provides a toolkit for normalizing, matching and
// specifying container platforms.
//
// Centered around OCI platform specifications, we define a string-based
// specifier syntax that can be used for user input. With a specifier, users
// only need to specify the parts of the platform that are relevant to their
// context, providing an operating system or architecture or both.
//
// How do I use this package?
//
// The vast majority of use cases should simply use the match function with
// user input. The first step is to parse a specifier into a matcher:
//
//	m, err := Parse("linux")
//	if err != nil { ... }
//
// Once you have a matcher, use it to match against the platform declared by a
// component, typically from an image or runtime. Since extracting an images
// platform is a little more involved, we'll use an example against the
// platform default:
//
//	if ok := m.Match(Default()); !ok { /* doesn't match */ }
//
// This can be composed in loops for resolving runtimes or used as a filter for
// fetch and select images.
//
// More details of the specifier syntax and platform spec follow.
//
// # Declaring Platform Support
//
// Components that have strict platform requirements should use the OCI
// platform specification to declare their support. Typically, this will be
// images and runtimes that should make these declaring which platform they
// support specifically. This looks roughly as follows:
//
//	  type Platform struct {
//		   Architecture string
//		   OS           string
//		   Variant      string
//	  }
//
// Most images and runtimes should at least set Architecture and OS, according
// to their GOARCH and GOOS values, respectively (follow the OCI image
// specification when in doubt). ARM should set variant under certain
// discussions, which are outlined below.
//
// # Platform Specifiers
//
// While the OCI platform specifications provide a tool for components to
// specify structured information, user input typically doesn't need the full
// context and much can be inferred. To solve this problem, we introduced
// "specifiers". A specifier has the format
// `<os>|<arch>|<os>/<arch>[/<variant>]`.  The user can provide either the
// operating system or the architecture or both.
//
// An example of a common specifier is `linux/amd64`. If the host has a default
// of runtime that matches this, the user can simply provide the component that
// matters. For example, if a image provides amd64 and arm64 support, the
// operating system, `linux` can be inferred, so they only have to provide
// `arm64` or `amd64`. Similar behavior is implemented for operating systems,
// where the architecture may be known but a runtime may support images from
// different operating systems.
//
// # Normalization
//
// Because not all users are familiar with the way the Go runtime represents
// platforms, several normalizations have been provided to make this package
// easier to user.
//
// The following are performed for architectures:
//
//	Value    Normalized
//	aarch64  arm64
//	armhf    arm
//	armel    arm/v6
//	i386     386
//	x86_64   amd64
//	x86-64   amd64
//
// We also normalize the operating system `macos` to `darwin`.
//
// # ARM Support
//
// To qualify ARM architecture, the Variant field is used to qualify the arm
// version. The most common arm version, v7, is represented without the variant
// unless it is explicitly provided. This is treated as equivalent to armhf. A
// previous architecture, armel, will be normalized to arm/v6.
//
// Similarly, the most common arm64 version v8, and most common amd64 version v1
// are represented without the variant.
//
// While these normalizations are provided, their support on arm platforms has
// not yet been fully implemented and tested.
package platforms

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	specifierRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	osRe        = regexp.MustCompile(`^([A-Za-z0-9_-]+)(?:\(([A-Za-z0-9_.%-]*)((?:\+[A-Za-z0-9_.%-]+)*)\))?$`)
)

// Platform is a type alias for convenience, so there is no need to import image-spec package everywhere.
type Platform = specs.Platform

// Matcher matches platforms specifications, provided by an image or runtime.
type Matcher interface {
	Match(platform specs.Platform) bool
}

// NewMatcher returns a simple matcher based on the provided platform
// specification. The returned matcher only looks for equality based on os,
// architecture and variant.
//
// One may implement their own matcher if this doesn't provide the required
// functionality.
//
// Applications should opt to use `Match` over directly parsing specifiers.
//
// For OSFeatures, this matcher will match if the platform to match has
// OSFeatures which are a subset of the OSFeatures of the platform
// provided to NewMatcher.
func NewMatcher(platform specs.Platform) Matcher {
	m := &matcher{
		Platform: Normalize(platform),
	}

	if platform.OS == "windows" {
		m.osvM = &windowsVersionMatcher{
			windowsOSVersion: getWindowsOSVersion(platform.OSVersion),
		}

		// In prior versions, the win32k os feature was not considered for matching,
		// strip out the win32k feature for comparison
		var stripped Matcher = windowsStripFeaturesMatcher{m}

		// In prior versions, on windows, the returned matcher implements a
		// MatchComprarer interface.
		// This preserves that behavior for backwards compatibility.
		//
		// TODO: This isn't actually used in this package, except for a test case,
		// which may have been an unintended side of some refactor.
		// It was likely intended to be used in `Ordered` but it is not since
		// `Less` that is implemented here ends up getting masked due to wrapping.
		if runtime.GOOS == "windows" {
			return &windowsMatchComparer{stripped}
		}
		return stripped
	}
	return m
}

type osVerMatcher interface {
	Match(string) bool
}

type matcher struct {
	specs.Platform
	osvM osVerMatcher
}

func (m *matcher) Match(platform specs.Platform) bool {
	normalized := Normalize(platform)
	if m.OS == normalized.OS &&
		m.Architecture == normalized.Architecture &&
		m.Variant == normalized.Variant &&
		m.matchOSVersion(platform) {
		if len(normalized.OSFeatures) == 0 {
			return true
		}
		if len(m.OSFeatures) >= len(normalized.OSFeatures) {
			// Ensure that normalized.OSFeatures is a subset of
			// m.OSFeatures
			j := 0
			for _, feature := range normalized.OSFeatures {
				found := false
				for ; j < len(m.OSFeatures); j++ {
					if feature == m.OSFeatures[j] {
						found = true
						j++
						break
					}
					// Since both lists are ordered, if the feature is less
					// than what is seen, it is not in the list
					if feature < m.OSFeatures[j] {
						return false
					}
				}
				if !found {
					return false
				}
			}
			return true
		}
	}
	return false
}

func (m *matcher) matchOSVersion(platform specs.Platform) bool {
	if m.osvM != nil {
		return m.osvM.Match(platform.OSVersion)
	}
	return true
}

func (m *matcher) String() string {
	return FormatAll(m.Platform)
}

// ParseAll parses a list of platform specifiers into a list of platform.
func ParseAll(specifiers []string) ([]specs.Platform, error) {
	platforms := make([]specs.Platform, len(specifiers))
	for i, s := range specifiers {
		p, err := Parse(s)
		if err != nil {
			return nil, fmt.Errorf("invalid platform %s: %w", s, err)
		}
		platforms[i] = p
	}
	return platforms, nil
}

// Parse parses the platform specifier syntax into a platform declaration.
//
// Platform specifiers are in the format `<os>[(<os options>)]|<arch>|<os>[(<os options>)]/<arch>[/<variant>]`.
// The minimum required information for a platform specifier is the operating
// system or architecture. The "os options" may be OSVersion which can be part of the OS
// like `windows(10.0.17763)`. When an OSVersion is specified, then specs.Platform.OSVersion is
// populated with that value, and an empty string otherwise. The "os options" may also include an
// array of OSFeatures, each feature prefixed with '+', without any other separator, and provided
// after the OSVersion when the OSVersion is specified. An "os options" with version and features
// is like `windows(10.0.17763+win32k)`.
// If there is only a single string (no slashes), the
// value will be matched against the known set of operating systems, then fall
// back to the known set of architectures. The missing component will be
// inferred based on the local environment.
func Parse(specifier string) (specs.Platform, error) {
	if strings.Contains(specifier, "*") {
		// TODO(stevvooe): need to work out exact wildcard handling
		return specs.Platform{}, fmt.Errorf("%q: wildcards not yet supported: %w", specifier, errInvalidArgument)
	}

	// Limit to 4 elements to prevent unbounded split
	parts := strings.SplitN(specifier, "/", 4)

	var p specs.Platform
	for i, part := range parts {
		if i == 0 {
			// First element is <os>[(<OSVersion>[+<OSFeature>]*)]
			osOptions := osRe.FindStringSubmatch(part)
			if osOptions == nil {
				return specs.Platform{}, fmt.Errorf("%q is an invalid OS component of %q: OSAndVersion specifier component must match %q: %w", part, specifier, osRe.String(), errInvalidArgument)
			}

			p.OS = normalizeOS(osOptions[1])
			osVersion, err := decodeOSOption(osOptions[2])
			if err != nil {
				return specs.Platform{}, fmt.Errorf("%q has an invalid OS version %q: %w", specifier, osOptions[2], err)
			}
			p.OSVersion = osVersion
			if osOptions[3] != "" {
				p.OSFeatures, err = parseOSFeatures(osOptions[3][1:])
				if err != nil {
					return specs.Platform{}, fmt.Errorf("%q has invalid OS features: %w", specifier, err)
				}
			}
		} else {
			if !specifierRe.MatchString(part) {
				return specs.Platform{}, fmt.Errorf("%q is an invalid component of %q: platform specifier component must match %q: %w", part, specifier, specifierRe.String(), errInvalidArgument)
			}
		}
	}

	switch len(parts) {
	case 1:
		// in this case, we will test that the value might be an OS (with or
		// without the optional OSVersion specified) and look it up.
		// If it is not known, we'll treat it as an architecture. Since
		// we have very little information about the platform here, we are
		// going to be a little more strict if we don't know about the argument
		// value.
		if isKnownOS(p.OS) {
			// picks a default architecture
			p.Architecture = runtime.GOARCH
			if p.Architecture == "arm" && cpuVariant() != "v7" {
				p.Variant = cpuVariant()
			}

			return p, nil
		}

		p.Architecture, p.Variant = normalizeArch(parts[0], "")
		if p.Architecture == "arm" && p.Variant == "v7" {
			p.Variant = ""
		}
		if isKnownArch(p.Architecture) {
			p.OS = runtime.GOOS
			return p, nil
		}

		return specs.Platform{}, fmt.Errorf("%q: unknown operating system or architecture: %w", specifier, errInvalidArgument)
	case 2:
		// In this case, we treat as a regular OS[(OSVersion)]/arch pair. We don't care
		// about whether or not we know of the platform.
		p.Architecture, p.Variant = normalizeArch(parts[1], "")
		if p.Architecture == "arm" && p.Variant == "v7" {
			p.Variant = ""
		}

		return p, nil
	case 3:
		// we have a fully specified variant, this is rare
		p.Architecture, p.Variant = normalizeArch(parts[1], parts[2])
		if p.Architecture == "arm64" && p.Variant == "" {
			p.Variant = "v8"
		}

		return p, nil
	}

	return specs.Platform{}, fmt.Errorf("%q: cannot parse platform specifier: %w", specifier, errInvalidArgument)
}

func parseOSFeatures(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}

	var features []string
	for raw := range strings.SplitSeq(s, "+") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil, fmt.Errorf("empty os feature: %w", errInvalidArgument)
		}
		feature, err := decodeOSOption(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid os feature %q: %w", raw, err)
		}
		if feature == "" {
			continue
		}
		features = append(features, feature)
	}

	return features, nil
}

// MustParse is like Parses but panics if the specifier cannot be parsed.
// Simplifies initialization of global variables.
func MustParse(specifier string) specs.Platform {
	p, err := Parse(specifier)
	if err != nil {
		panic("platform: Parse(" + strconv.Quote(specifier) + "): " + err.Error())
	}
	return p
}

// Format returns a string specifier from the provided platform specification.
func Format(platform specs.Platform) string {
	if platform.OS == "" {
		return "unknown"
	}

	return path.Join(platform.OS, platform.Architecture, platform.Variant)
}

// FormatAll returns a string specifier that also includes the OSVersion from the
// provided platform specification.
func FormatAll(platform specs.Platform) string {
	if platform.OS == "" {
		return "unknown"
	}
	if platform.OSVersion == "" && len(platform.OSFeatures) == 0 {
		return path.Join(platform.OS, platform.Architecture, platform.Variant)
	}

	var b strings.Builder
	b.WriteString(platform.OS)
	osv := encodeOSOption(platform.OSVersion)
	formatted := formatOSFeatures(platform.OSFeatures)
	if osv != "" || formatted != "" {
		b.Grow(len(osv) + len(formatted) + 3) // parens + maybe '+'
		b.WriteByte('(')
		if osv != "" {
			b.WriteString(osv)
		}
		if formatted != "" {
			b.WriteByte('+')
			b.WriteString(formatted)
		}
		b.WriteByte(')')
	}

	return path.Join(b.String(), platform.Architecture, platform.Variant)
}

func formatOSFeatures(features []string) string {
	if len(features) == 0 {
		return ""
	}

	if !slices.IsSorted(features) {
		features = slices.Clone(features)
		slices.Sort(features)
	}
	var b strings.Builder
	var wrote bool
	var prev string
	for _, f := range features {
		if f == "" || f == prev {
			// skip empty and duplicate values
			continue
		}
		prev = f
		if wrote {
			b.WriteByte('+')
		}
		b.WriteString(encodeOSOption(f))
		wrote = true
	}
	return b.String()
}

// osOptionReplacer encodes characters in OS option values (version and
// features) that are ambiguous with the format syntax. The percent sign
// must be replaced first to avoid double-encoding.
var osOptionReplacer = strings.NewReplacer(
	"%", "%25",
	"+", "%2B",
	"(", "%28",
	")", "%29",
	"/", "%2F",
)

func encodeOSOption(v string) string {
	return osOptionReplacer.Replace(v)
}

func decodeOSOption(v string) (string, error) {
	if strings.Contains(v, "%") {
		return url.PathUnescape(v)
	}
	return v, nil
}

// Normalize validates and translate the platform to the canonical value.
//
// For example, if "Aarch64" is encountered, we change it to "arm64" or if
// "x86_64" is encountered, it becomes "amd64".
func Normalize(platform specs.Platform) specs.Platform {
	platform.OS = normalizeOS(platform.OS)
	platform.Architecture, platform.Variant = normalizeArch(platform.Architecture, platform.Variant)
	if len(platform.OSFeatures) > 0 {
		platform.OSFeatures = slices.Clone(platform.OSFeatures)
		slices.Sort(platform.OSFeatures)
		platform.OSFeatures = slices.Compact(platform.OSFeatures)
	}

	return platform
}
