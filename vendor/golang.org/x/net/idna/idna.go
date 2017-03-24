// Copied from the golang.org/x/text repo; DO NOT EDIT

// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package idna implements IDNA2008 using the compatibility processing
// defined by UTS (Unicode Technical Standard) #46, which defines a standard to
// deal with the transition from IDNA2003.
//
// IDNA2008 (Internationalized Domain Names for Applications), is defined in RFC
// 5890, RFC 5891, RFC 5892, RFC 5893 and RFC 5894.
// UTS #46 is defined in http://www.unicode.org/reports/tr46.
// See http://unicode.org/cldr/utility/idna.jsp for a visualization of the
// differences between these two standards.
package idna // import "golang.org/x/net/idna"

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/secure/bidirule"
	"golang.org/x/text/unicode/norm"
)

// NOTE: Unlike common practice in Go APIs, the functions will return a
// sanitized domain name in case of errors. Browsers sometimes use a partially
// evaluated string as lookup.
// TODO: the current error handling is, in my opinion, the least opinionated.
// Other strategies are also viable, though:
// Option 1) Return an empty string in case of error, but allow the user to
//    specify explicitly which errors to ignore.
// Option 2) Return the partially evaluated string if it is itself a valid
//    string, otherwise return the empty string in case of error.
// Option 3) Option 1 and 2.
// Option 4) Always return an empty string for now and implement Option 1 as
//    needed, and document that the return string may not be empty in case of
//    error in the future.
// I think Option 1 is best, but it is quite opinionated.

// ToASCII converts a domain or domain label to its ASCII form. For example,
// ToASCII("bücher.example.com") is "xn--bcher-kva.example.com", and
// ToASCII("golang") is "golang". If an error is encountered it will return
// an error and a (partially) processed result.
func ToASCII(s string) (string, error) {
	return Resolve.process(s, true)
}

// ToUnicode converts a domain or domain label to its Unicode form. For example,
// ToUnicode("xn--bcher-kva.example.com") is "bücher.example.com", and
// ToUnicode("golang") is "golang". If an error is encountered it will return
// an error and a (partially) processed result.
func ToUnicode(s string) (string, error) {
	return NonTransitional.process(s, false)
}

// An Option configures a Profile at creation time.
type Option func(*options)

// Transitional sets a Profile to use the Transitional mapping as defined
// in UTS #46.
func Transitional(transitional bool) Option {
	return func(o *options) { o.transitional = true }
}

// VerifyDNSLength sets whether a Profile should fail if any of the IDN parts
// are longer than allowed by the RFC.
func VerifyDNSLength(verify bool) Option {
	return func(o *options) { o.verifyDNSLength = verify }
}

// IgnoreSTD3Rules sets whether ASCII characters outside the A-Z, a-z, 0-9 and
// the hyphen should be allowed. By default this is not allowed, but IDNA2003,
// and as a consequence UTS #46, allows this to be overridden to support
// browsers that allow characters outside this range, for example a '_' (U+005F
// LOW LINE). See http://www.rfc- editor.org/std/std3.txt for more details.
func IgnoreSTD3Rules(ignore bool) Option {
	return func(o *options) { o.ignoreSTD3Rules = ignore }
}

type options struct {
	transitional    bool
	ignoreSTD3Rules bool
	verifyDNSLength bool
}

// A Profile defines the configuration of a IDNA mapper.
type Profile struct {
	options
}

func apply(o *options, opts []Option) {
	for _, f := range opts {
		f(o)
	}
}

// New creates a new Profile.
// With no options, the returned profile is the non-transitional profile as
// defined in UTS #46.
func New(o ...Option) *Profile {
	p := &Profile{}
	apply(&p.options, o)
	return p
}

// ToASCII converts a domain or domain label to its ASCII form. For example,
// ToASCII("bücher.example.com") is "xn--bcher-kva.example.com", and
// ToASCII("golang") is "golang". If an error is encountered it will return
// an error and a (partially) processed result.
func (p *Profile) ToASCII(s string) (string, error) {
	return p.process(s, true)
}

// ToUnicode converts a domain or domain label to its Unicode form. For example,
// ToUnicode("xn--bcher-kva.example.com") is "bücher.example.com", and
// ToUnicode("golang") is "golang". If an error is encountered it will return
// an error and a (partially) processed result.
func (p *Profile) ToUnicode(s string) (string, error) {
	pp := *p
	pp.transitional = false
	return pp.process(s, false)
}

// String reports a string with a description of the profile for debugging
// purposes. The string format may change with different versions.
func (p *Profile) String() string {
	s := ""
	if p.transitional {
		s = "Transitional"
	} else {
		s = "NonTransitional"
	}
	if p.ignoreSTD3Rules {
		s += ":NoSTD3Rules"
	}
	return s
}

var (
	// Resolve is the recommended profile for resolving domain names.
	// The configuration of this profile may change over time.
	Resolve = resolve

	// Display is the recommended profile for displaying domain names.
	// The configuration of this profile may change over time.
	Display = display

	// NonTransitional defines a profile that implements the Transitional
	// mapping as defined in UTS #46 with no additional constraints.
	NonTransitional = nonTransitional

	resolve         = &Profile{options{transitional: true}}
	display         = &Profile{}
	nonTransitional = &Profile{}

	// TODO: profiles
	// V2008: strict IDNA2008
	// Register: recommended for approving domain names: nontransitional, but
	// bundle or block deviation characters.
)

type labelError struct{ label, code_ string }

func (e labelError) code() string { return e.code_ }
func (e labelError) Error() string {
	return fmt.Sprintf("idna: invalid label %q", e.label)
}

type runeError rune

func (e runeError) code() string { return "P1" }
func (e runeError) Error() string {
	return fmt.Sprintf("idna: disallowed rune %U", e)
}

// process implements the algorithm described in section 4 of UTS #46,
// see http://www.unicode.org/reports/tr46.
func (p *Profile) process(s string, toASCII bool) (string, error) {
	var (
		b    []byte
		err  error
		k, i int
	)
	for i < len(s) {
		v, sz := trie.lookupString(s[i:])
		start := i
		i += sz
		// Copy bytes not copied so far.
		switch p.simplify(info(v).category()) {
		case valid:
			continue
		case disallowed:
			if err == nil {
				r, _ := utf8.DecodeRuneInString(s[i:])
				err = runeError(r)
			}
			continue
		case mapped, deviation:
			b = append(b, s[k:start]...)
			b = info(v).appendMapping(b, s[start:i])
		case ignored:
			b = append(b, s[k:start]...)
			// drop the rune
		case unknown:
			b = append(b, s[k:start]...)
			b = append(b, "\ufffd"...)
		}
		k = i
	}
	if k == 0 {
		// No changes so far.
		s = norm.NFC.String(s)
	} else {
		b = append(b, s[k:]...)
		if norm.NFC.QuickSpan(b) != len(b) {
			b = norm.NFC.Bytes(b)
		}
		// TODO: the punycode converters require strings as input.
		s = string(b)
	}
	// Remove leading empty labels
	for ; len(s) > 0 && s[0] == '.'; s = s[1:] {
	}
	if s == "" {
		return "", &labelError{s, "A4"}
	}
	labels := labelIter{orig: s}
	for ; !labels.done(); labels.next() {
		label := labels.label()
		if label == "" {
			// Empty labels are not okay. The label iterator skips the last
			// label if it is empty.
			if err == nil {
				err = &labelError{s, "A4"}
			}
			continue
		}
		if strings.HasPrefix(label, acePrefix) {
			u, err2 := decode(label[len(acePrefix):])
			if err2 != nil {
				if err == nil {
					err = err2
				}
				// Spec says keep the old label.
				continue
			}
			labels.set(u)
			if err == nil {
				err = p.validateFromPunycode(u)
			}
			if err == nil {
				err = NonTransitional.validate(u)
			}
		} else if err == nil {
			err = p.validate(label)
		}
	}
	if toASCII {
		for labels.reset(); !labels.done(); labels.next() {
			label := labels.label()
			if !ascii(label) {
				a, err2 := encode(acePrefix, label)
				if err == nil {
					err = err2
				}
				label = a
				labels.set(a)
			}
			n := len(label)
			if p.verifyDNSLength && err == nil && (n == 0 || n > 63) {
				err = &labelError{label, "A4"}
			}
		}
	}
	s = labels.result()
	if toASCII && p.verifyDNSLength && err == nil {
		// Compute the length of the domain name minus the root label and its dot.
		n := len(s)
		if n > 0 && s[n-1] == '.' {
			n--
		}
		if len(s) < 1 || n > 253 {
			err = &labelError{s, "A4"}
		}
	}
	return s, err
}

// A labelIter allows iterating over domain name labels.
type labelIter struct {
	orig     string
	slice    []string
	curStart int
	curEnd   int
	i        int
}

func (l *labelIter) reset() {
	l.curStart = 0
	l.curEnd = 0
	l.i = 0
}

func (l *labelIter) done() bool {
	return l.curStart >= len(l.orig)
}

func (l *labelIter) result() string {
	if l.slice != nil {
		return strings.Join(l.slice, ".")
	}
	return l.orig
}

func (l *labelIter) label() string {
	if l.slice != nil {
		return l.slice[l.i]
	}
	p := strings.IndexByte(l.orig[l.curStart:], '.')
	l.curEnd = l.curStart + p
	if p == -1 {
		l.curEnd = len(l.orig)
	}
	return l.orig[l.curStart:l.curEnd]
}

// next sets the value to the next label. It skips the last label if it is empty.
func (l *labelIter) next() {
	l.i++
	if l.slice != nil {
		if l.i >= len(l.slice) || l.i == len(l.slice)-1 && l.slice[l.i] == "" {
			l.curStart = len(l.orig)
		}
	} else {
		l.curStart = l.curEnd + 1
		if l.curStart == len(l.orig)-1 && l.orig[l.curStart] == '.' {
			l.curStart = len(l.orig)
		}
	}
}

func (l *labelIter) set(s string) {
	if l.slice == nil {
		l.slice = strings.Split(l.orig, ".")
	}
	l.slice[l.i] = s
}

// acePrefix is the ASCII Compatible Encoding prefix.
const acePrefix = "xn--"

func (p *Profile) simplify(cat category) category {
	switch cat {
	case disallowedSTD3Mapped:
		if !p.ignoreSTD3Rules {
			cat = disallowed
		} else {
			cat = mapped
		}
	case disallowedSTD3Valid:
		if !p.ignoreSTD3Rules {
			cat = disallowed
		} else {
			cat = valid
		}
	case deviation:
		if !p.transitional {
			cat = valid
		}
	case validNV8, validXV8:
		// TODO: handle V2008
		cat = valid
	}
	return cat
}

func (p *Profile) validateFromPunycode(s string) error {
	if !norm.NFC.IsNormalString(s) {
		return &labelError{s, "V1"}
	}
	for i := 0; i < len(s); {
		v, sz := trie.lookupString(s[i:])
		if c := p.simplify(info(v).category()); c != valid && c != deviation {
			return &labelError{s, "V6"}
		}
		i += sz
	}
	return nil
}

const (
	zwnj = "\u200c"
	zwj  = "\u200d"
)

type joinState int8

const (
	stateStart joinState = iota
	stateVirama
	stateBefore
	stateBeforeVirama
	stateAfter
	stateFAIL
)

var joinStates = [][numJoinTypes]joinState{
	stateStart: {
		joiningL:   stateBefore,
		joiningD:   stateBefore,
		joinZWNJ:   stateFAIL,
		joinZWJ:    stateFAIL,
		joinVirama: stateVirama,
	},
	stateVirama: {
		joiningL: stateBefore,
		joiningD: stateBefore,
	},
	stateBefore: {
		joiningL:   stateBefore,
		joiningD:   stateBefore,
		joiningT:   stateBefore,
		joinZWNJ:   stateAfter,
		joinZWJ:    stateFAIL,
		joinVirama: stateBeforeVirama,
	},
	stateBeforeVirama: {
		joiningL: stateBefore,
		joiningD: stateBefore,
		joiningT: stateBefore,
	},
	stateAfter: {
		joiningL:   stateFAIL,
		joiningD:   stateBefore,
		joiningT:   stateAfter,
		joiningR:   stateStart,
		joinZWNJ:   stateFAIL,
		joinZWJ:    stateFAIL,
		joinVirama: stateAfter, // no-op as we can't accept joiners here
	},
	stateFAIL: {
		0:          stateFAIL,
		joiningL:   stateFAIL,
		joiningD:   stateFAIL,
		joiningT:   stateFAIL,
		joiningR:   stateFAIL,
		joinZWNJ:   stateFAIL,
		joinZWJ:    stateFAIL,
		joinVirama: stateFAIL,
	},
}

// validate validates the criteria from Section 4.1. Item 1, 4, and 6 are
// already implicitly satisfied by the overall implementation.
func (p *Profile) validate(s string) error {
	if len(s) > 4 && s[2] == '-' && s[3] == '-' {
		return &labelError{s, "V2"}
	}
	if s[0] == '-' || s[len(s)-1] == '-' {
		return &labelError{s, "V3"}
	}
	// TODO: merge the use of this in the trie.
	v, sz := trie.lookupString(s)
	x := info(v)
	if x.isModifier() {
		return &labelError{s, "V5"}
	}
	if !bidirule.ValidString(s) {
		return &labelError{s, "B"}
	}
	// Quickly return in the absence of zero-width (non) joiners.
	if strings.Index(s, zwj) == -1 && strings.Index(s, zwnj) == -1 {
		return nil
	}
	st := stateStart
	for i := 0; ; {
		jt := x.joinType()
		if s[i:i+sz] == zwj {
			jt = joinZWJ
		} else if s[i:i+sz] == zwnj {
			jt = joinZWNJ
		}
		st = joinStates[st][jt]
		if x.isViramaModifier() {
			st = joinStates[st][joinVirama]
		}
		if i += sz; i == len(s) {
			break
		}
		v, sz = trie.lookupString(s[i:])
		x = info(v)
	}
	if st == stateFAIL || st == stateAfter {
		return &labelError{s, "C"}
	}
	return nil
}

func ascii(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}
