// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package strfmt

// ISODurationPolicy is a compile-time policy selecting the parsing behaviour of an [ISODuration].
//
// It is a phantom type parameter: a zero-size value whose single method yields the parser configuration.
// See [DurationStrict] and [DurationLenient].
type ISODurationPolicy interface {
	isoDurationConfig() isoDurationConfig
}

// isoDurationConfig holds the leniency knobs of the ISO 8601 duration parser.
//
// The zero value is the strict RFC 3339 Appendix A grammar (the JSON Schema "duration" format): every field false.
type isoDurationConfig struct {
	allowFraction  bool // accept a decimal fraction on the least significant component
	allowSign      bool // accept a leading '+' or '-'
	allowSpace     bool // tolerate surrounding whitespace
	weekCombinable bool // allow the 'W' designator to combine with other components
	relaxAnchoring bool // allow gaps in the component chain (e.g. P1Y2D, PT1H2S)
}

// DurationStrict is the strict RFC 3339 Appendix A policy.
//
// This is the grammar that the JSON Schema "duration" format is defined against (draft 2020):
// no fraction, no sign, no whitespace, strict ordering and anchoring, and an exclusive 'W' designator.
type DurationStrict struct{}

func (DurationStrict) isoDurationConfig() isoDurationConfig { return isoDurationConfig{} }

// DurationLenient relaxes every strictness knob.
//
// It accepts a decimal fraction on the least significant component, a leading sign, surrounding whitespace,
// component chains with gaps (e.g. P1Y2D), and a 'W' designator combined with other components.
//
// It remains ordered (P2D1Y is still rejected).
type DurationLenient struct{}

func (DurationLenient) isoDurationConfig() isoDurationConfig {
	return isoDurationConfig{
		allowFraction:  true,
		allowSign:      true,
		allowSpace:     true,
		weekCombinable: true,
		relaxAnchoring: true,
	}
}

// ISODurationOption relaxes the strict parser on the explicit [ParseISO8601Duration] path, for programmatic callers.
//
// It has no effect on the registry / struct-field decode path, which is governed by the type's policy.
type ISODurationOption func(*isoDurationConfig)

// WithISOFractions accepts a decimal fraction on the least significant component.
func WithISOFractions() ISODurationOption {
	return func(c *isoDurationConfig) { c.allowFraction = true }
}

// WithISOSign accepts a leading '+' or '-'.
func WithISOSign() ISODurationOption { return func(c *isoDurationConfig) { c.allowSign = true } }

// WithISOSpace tolerates surrounding whitespace.
func WithISOSpace() ISODurationOption { return func(c *isoDurationConfig) { c.allowSpace = true } }

// WithISOWeekCombinable allows the 'W' designator to combine with other components.
func WithISOWeekCombinable() ISODurationOption {
	return func(c *isoDurationConfig) { c.weekCombinable = true }
}

// WithISORelaxedAnchoring allows gaps in the component chain (e.g. P1Y2D, PT1H2S).
func WithISORelaxedAnchoring() ISODurationOption {
	return func(c *isoDurationConfig) { c.relaxAnchoring = true }
}

// WithISOLenient relaxes every strictness knob (see [DurationLenient]).
func WithISOLenient() ISODurationOption {
	return func(c *isoDurationConfig) { *c = DurationLenient{}.isoDurationConfig() }
}
