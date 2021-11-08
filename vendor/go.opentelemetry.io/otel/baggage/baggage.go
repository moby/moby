// Copyright The OpenTelemetry Authors
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

package baggage // import "go.opentelemetry.io/otel/baggage"

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/internal/baggage"
)

const (
	maxMembers               = 180
	maxBytesPerMembers       = 4096
	maxBytesPerBaggageString = 8192

	listDelimiter     = ","
	keyValueDelimiter = "="
	propertyDelimiter = ";"

	keyDef      = `([\x21\x23-\x27\x2A\x2B\x2D\x2E\x30-\x39\x41-\x5a\x5e-\x7a\x7c\x7e]+)`
	valueDef    = `([\x21\x23-\x2b\x2d-\x3a\x3c-\x5B\x5D-\x7e]*)`
	keyValueDef = `\s*` + keyDef + `\s*` + keyValueDelimiter + `\s*` + valueDef + `\s*`
)

var (
	keyRe      = regexp.MustCompile(`^` + keyDef + `$`)
	valueRe    = regexp.MustCompile(`^` + valueDef + `$`)
	propertyRe = regexp.MustCompile(`^(?:\s*` + keyDef + `\s*|` + keyValueDef + `)$`)
)

var (
	errInvalidKey      = errors.New("invalid key")
	errInvalidValue    = errors.New("invalid value")
	errInvalidProperty = errors.New("invalid baggage list-member property")
	errInvalidMember   = errors.New("invalid baggage list-member")
	errMemberNumber    = errors.New("too many list-members in baggage-string")
	errMemberBytes     = errors.New("list-member too large")
	errBaggageBytes    = errors.New("baggage-string too large")
)

// Property is an additional metadata entry for a baggage list-member.
type Property struct {
	key, value string

	// hasValue indicates if a zero-value value means the property does not
	// have a value or if it was the zero-value.
	hasValue bool
}

func NewKeyProperty(key string) (Property, error) {
	p := Property{}
	if !keyRe.MatchString(key) {
		return p, fmt.Errorf("%w: %q", errInvalidKey, key)
	}
	p.key = key
	return p, nil
}

func NewKeyValueProperty(key, value string) (Property, error) {
	p := Property{}
	if !keyRe.MatchString(key) {
		return p, fmt.Errorf("%w: %q", errInvalidKey, key)
	}
	if !valueRe.MatchString(value) {
		return p, fmt.Errorf("%w: %q", errInvalidValue, value)
	}
	p.key = key
	p.value = value
	p.hasValue = true
	return p, nil
}

// parseProperty attempts to decode a Property from the passed string. It
// returns an error if the input is invalid according to the W3C Baggage
// specification.
func parseProperty(property string) (Property, error) {
	p := Property{}
	if property == "" {
		return p, nil
	}

	match := propertyRe.FindStringSubmatch(property)
	if len(match) != 4 {
		return p, fmt.Errorf("%w: %q", errInvalidProperty, property)
	}

	if match[1] != "" {
		p.key = match[1]
	} else {
		p.key = match[2]
		p.value = match[3]
		p.hasValue = true
	}
	return p, nil
}

// validate ensures p conforms to the W3C Baggage specification, returning an
// error otherwise.
func (p Property) validate() error {
	errFunc := func(err error) error {
		return fmt.Errorf("invalid property: %w", err)
	}

	if !keyRe.MatchString(p.key) {
		return errFunc(fmt.Errorf("%w: %q", errInvalidKey, p.key))
	}
	if p.hasValue && !valueRe.MatchString(p.value) {
		return errFunc(fmt.Errorf("%w: %q", errInvalidValue, p.value))
	}
	if !p.hasValue && p.value != "" {
		return errFunc(errors.New("inconsistent value"))
	}
	return nil
}

// Key returns the Property key.
func (p Property) Key() string {
	return p.key
}

// Value returns the Property value. Additionally a boolean value is returned
// indicating if the returned value is the empty if the Property has a value
// that is empty or if the value is not set.
func (p Property) Value() (string, bool) {
	return p.value, p.hasValue
}

// String encodes Property into a string compliant with the W3C Baggage
// specification.
func (p Property) String() string {
	if p.hasValue {
		return fmt.Sprintf("%s%s%v", p.key, keyValueDelimiter, p.value)
	}
	return p.key
}

type properties []Property

func fromInternalProperties(iProps []baggage.Property) properties {
	if len(iProps) == 0 {
		return nil
	}

	props := make(properties, len(iProps))
	for i, p := range iProps {
		props[i] = Property{
			key:      p.Key,
			value:    p.Value,
			hasValue: p.HasValue,
		}
	}
	return props
}

func (p properties) asInternal() []baggage.Property {
	if len(p) == 0 {
		return nil
	}

	iProps := make([]baggage.Property, len(p))
	for i, prop := range p {
		iProps[i] = baggage.Property{
			Key:      prop.key,
			Value:    prop.value,
			HasValue: prop.hasValue,
		}
	}
	return iProps
}

func (p properties) Copy() properties {
	if len(p) == 0 {
		return nil
	}

	props := make(properties, len(p))
	copy(props, p)
	return props
}

// validate ensures each Property in p conforms to the W3C Baggage
// specification, returning an error otherwise.
func (p properties) validate() error {
	for _, prop := range p {
		if err := prop.validate(); err != nil {
			return err
		}
	}
	return nil
}

// String encodes properties into a string compliant with the W3C Baggage
// specification.
func (p properties) String() string {
	props := make([]string, len(p))
	for i, prop := range p {
		props[i] = prop.String()
	}
	return strings.Join(props, propertyDelimiter)
}

// Member is a list-member of a baggage-string as defined by the W3C Baggage
// specification.
type Member struct {
	key, value string
	properties properties
}

// NewMember returns a new Member from the passed arguments. An error is
// returned if the created Member would be invalid according to the W3C
// Baggage specification.
func NewMember(key, value string, props ...Property) (Member, error) {
	m := Member{key: key, value: value, properties: properties(props).Copy()}
	if err := m.validate(); err != nil {
		return Member{}, err
	}

	return m, nil
}

// parseMember attempts to decode a Member from the passed string. It returns
// an error if the input is invalid according to the W3C Baggage
// specification.
func parseMember(member string) (Member, error) {
	if n := len(member); n > maxBytesPerMembers {
		return Member{}, fmt.Errorf("%w: %d", errMemberBytes, n)
	}

	var (
		key, value string
		props      properties
	)

	parts := strings.SplitN(member, propertyDelimiter, 2)
	switch len(parts) {
	case 2:
		// Parse the member properties.
		for _, pStr := range strings.Split(parts[1], propertyDelimiter) {
			p, err := parseProperty(pStr)
			if err != nil {
				return Member{}, err
			}
			props = append(props, p)
		}
		fallthrough
	case 1:
		// Parse the member key/value pair.

		// Take into account a value can contain equal signs (=).
		kv := strings.SplitN(parts[0], keyValueDelimiter, 2)
		if len(kv) != 2 {
			return Member{}, fmt.Errorf("%w: %q", errInvalidMember, member)
		}
		// "Leading and trailing whitespaces are allowed but MUST be trimmed
		// when converting the header into a data structure."
		key, value = strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
		if !keyRe.MatchString(key) {
			return Member{}, fmt.Errorf("%w: %q", errInvalidKey, key)
		}
		if !valueRe.MatchString(value) {
			return Member{}, fmt.Errorf("%w: %q", errInvalidValue, value)
		}
	default:
		// This should never happen unless a developer has changed the string
		// splitting somehow. Panic instead of failing silently and allowing
		// the bug to slip past the CI checks.
		panic("failed to parse baggage member")
	}

	return Member{key: key, value: value, properties: props}, nil
}

// validate ensures m conforms to the W3C Baggage specification, returning an
// error otherwise.
func (m Member) validate() error {
	if !keyRe.MatchString(m.key) {
		return fmt.Errorf("%w: %q", errInvalidKey, m.key)
	}
	if !valueRe.MatchString(m.value) {
		return fmt.Errorf("%w: %q", errInvalidValue, m.value)
	}
	return m.properties.validate()
}

// Key returns the Member key.
func (m Member) Key() string { return m.key }

// Value returns the Member value.
func (m Member) Value() string { return m.value }

// Properties returns a copy of the Member properties.
func (m Member) Properties() []Property { return m.properties.Copy() }

// String encodes Member into a string compliant with the W3C Baggage
// specification.
func (m Member) String() string {
	// A key is just an ASCII string, but a value is URL encoded UTF-8.
	s := fmt.Sprintf("%s%s%s", m.key, keyValueDelimiter, url.QueryEscape(m.value))
	if len(m.properties) > 0 {
		s = fmt.Sprintf("%s%s%s", s, propertyDelimiter, m.properties.String())
	}
	return s
}

// Baggage is a list of baggage members representing the baggage-string as
// defined by the W3C Baggage specification.
type Baggage struct { //nolint:golint
	list baggage.List
}

// New returns a new valid Baggage. It returns an error if the passed members
// are invalid according to the W3C Baggage specification or if it results in
// a Baggage exceeding limits set in that specification.
func New(members ...Member) (Baggage, error) {
	if len(members) == 0 {
		return Baggage{}, nil
	}

	b := make(baggage.List)
	for _, m := range members {
		if err := m.validate(); err != nil {
			return Baggage{}, err
		}
		// OpenTelemetry resolves duplicates by last-one-wins.
		b[m.key] = baggage.Item{
			Value:      m.value,
			Properties: m.properties.asInternal(),
		}
	}

	// Check member numbers after deduplicating.
	if len(b) > maxMembers {
		return Baggage{}, errMemberNumber
	}

	bag := Baggage{b}
	if n := len(bag.String()); n > maxBytesPerBaggageString {
		return Baggage{}, fmt.Errorf("%w: %d", errBaggageBytes, n)
	}

	return bag, nil
}

// Parse attempts to decode a baggage-string from the passed string. It
// returns an error if the input is invalid according to the W3C Baggage
// specification.
//
// If there are duplicate list-members contained in baggage, the last one
// defined (reading left-to-right) will be the only one kept. This diverges
// from the W3C Baggage specification which allows duplicate list-members, but
// conforms to the OpenTelemetry Baggage specification.
func Parse(bStr string) (Baggage, error) {
	if bStr == "" {
		return Baggage{}, nil
	}

	if n := len(bStr); n > maxBytesPerBaggageString {
		return Baggage{}, fmt.Errorf("%w: %d", errBaggageBytes, n)
	}

	b := make(baggage.List)
	for _, memberStr := range strings.Split(bStr, listDelimiter) {
		m, err := parseMember(memberStr)
		if err != nil {
			return Baggage{}, err
		}
		// OpenTelemetry resolves duplicates by last-one-wins.
		b[m.key] = baggage.Item{
			Value:      m.value,
			Properties: m.properties.asInternal(),
		}
	}

	// OpenTelemetry does not allow for duplicate list-members, but the W3C
	// specification does. Now that we have deduplicated, ensure the baggage
	// does not exceed list-member limits.
	if len(b) > maxMembers {
		return Baggage{}, errMemberNumber
	}

	return Baggage{b}, nil
}

// Member returns the baggage list-member identified by key.
//
// If there is no list-member matching the passed key the returned Member will
// be a zero-value Member.
func (b Baggage) Member(key string) Member {
	v, ok := b.list[key]
	if !ok {
		// We do not need to worry about distiguising between the situation
		// where a zero-valued Member is included in the Baggage because a
		// zero-valued Member is invalid according to the W3C Baggage
		// specification (it has an empty key).
		return Member{}
	}

	return Member{
		key:        key,
		value:      v.Value,
		properties: fromInternalProperties(v.Properties),
	}
}

// Members returns all the baggage list-members.
// The order of the returned list-members does not have significance.
func (b Baggage) Members() []Member {
	if len(b.list) == 0 {
		return nil
	}

	members := make([]Member, 0, len(b.list))
	for k, v := range b.list {
		members = append(members, Member{
			key:        k,
			value:      v.Value,
			properties: fromInternalProperties(v.Properties),
		})
	}
	return members
}

// SetMember returns a copy the Baggage with the member included. If the
// baggage contains a Member with the same key the existing Member is
// replaced.
//
// If member is invalid according to the W3C Baggage specification, an error
// is returned with the original Baggage.
func (b Baggage) SetMember(member Member) (Baggage, error) {
	if err := member.validate(); err != nil {
		return b, fmt.Errorf("%w: %s", errInvalidMember, err)
	}

	n := len(b.list)
	if _, ok := b.list[member.key]; !ok {
		n++
	}
	list := make(baggage.List, n)

	for k, v := range b.list {
		// Do not copy if we are just going to overwrite.
		if k == member.key {
			continue
		}
		list[k] = v
	}

	list[member.key] = baggage.Item{
		Value:      member.value,
		Properties: member.properties.asInternal(),
	}

	return Baggage{list: list}, nil
}

// DeleteMember returns a copy of the Baggage with the list-member identified
// by key removed.
func (b Baggage) DeleteMember(key string) Baggage {
	n := len(b.list)
	if _, ok := b.list[key]; ok {
		n--
	}
	list := make(baggage.List, n)

	for k, v := range b.list {
		if k == key {
			continue
		}
		list[k] = v
	}

	return Baggage{list: list}
}

// Len returns the number of list-members in the Baggage.
func (b Baggage) Len() int {
	return len(b.list)
}

// String encodes Baggage into a string compliant with the W3C Baggage
// specification. The returned string will be invalid if the Baggage contains
// any invalid list-members.
func (b Baggage) String() string {
	members := make([]string, 0, len(b.list))
	for k, v := range b.list {
		members = append(members, Member{
			key:        k,
			value:      v.Value,
			properties: fromInternalProperties(v.Properties),
		}.String())
	}
	return strings.Join(members, listDelimiter)
}
