package interpolation

import (
	"fmt"
	"strings"

	"github.com/docker/stacks/pkg/compose/template"
	"github.com/pkg/errors"
)

// Options supported by Interpolate
type Options struct {
	// LookupValue from a key
	LookupValue LookupValue
	// TypeCastMapping maps key paths to functions to cast to a type
	TypeCastMapping map[Path]Cast
	// Substitution function to use
	Substitute func(string, template.Mapping) (string, error)
}

// LookupValue is a function which maps from variable names to values.
// Returns the value as a string and a bool indicating whether
// the value is present, to distinguish between an empty string
// and the absence of a value.
type LookupValue func(key string) (string, bool)

// Cast a value to a new type, or return an error if the value can't be cast
type Cast func(value string) (interface{}, error)

// Interpolate replaces variables in a string with the values from a mapping
func Interpolate(config map[string]interface{}, opts Options) (map[string]interface{}, error) {
	if opts.LookupValue == nil {
		return nil, fmt.Errorf("missing LookupValue helper function")
	}
	if opts.TypeCastMapping == nil {
		opts.TypeCastMapping = make(map[Path]Cast)
	}
	if opts.Substitute == nil {
		opts.Substitute = template.Substitute
	}

	out := map[string]interface{}{}

	for key, value := range config {
		interpolatedValue, err := recursiveInterpolate(value, NewPath(key), opts)
		if err != nil {
			return out, err
		}
		out[key] = interpolatedValue
	}

	return out, nil
}

func recursiveInterpolate(value interface{}, path Path, opts Options) (interface{}, error) {
	switch value := value.(type) {

	case string:
		newValue, err := opts.Substitute(value, template.Mapping(opts.LookupValue))
		if err != nil || newValue == value {
			return value, newPathError(path, err)
		}
		caster, ok := opts.getCasterForPath(path)
		if !ok {
			return newValue, nil
		}
		casted, err := caster(newValue)
		return casted, newPathError(path, errors.Wrap(err, "failed to cast to expected type"))

	case map[string]interface{}:
		out := map[string]interface{}{}
		for key, elem := range value {
			interpolatedElem, err := recursiveInterpolate(elem, path.Next(key), opts)
			if err != nil {
				return nil, err
			}
			out[key] = interpolatedElem
		}
		return out, nil

	case []interface{}:
		out := make([]interface{}, len(value))
		for i, elem := range value {
			interpolatedElem, err := recursiveInterpolate(elem, path.Next(PathMatchList), opts)
			if err != nil {
				return nil, err
			}
			out[i] = interpolatedElem
		}
		return out, nil

	default:
		return value, nil

	}
}

func newPathError(path Path, err error) error {
	switch err := err.(type) {
	case nil:
		return nil
	case *template.InvalidTemplateError:
		return errors.Errorf(
			"invalid interpolation format for %s: %#v. You may need to escape any $ with another $.",
			path, err.Template)
	default:
		return errors.Wrapf(err, "error while interpolating %s", path)
	}
}

const pathSeparator = "."

// PathMatchAll is a token used as part of a Path to match any key at that level
// in the nested structure
const PathMatchAll = "*"

// PathMatchList is a token used as part of a Path to match items in a list
const PathMatchList = "[]"

// Path is a dotted path of keys to a value in a nested mapping structure. A *
// section in a path will match any key in the mapping structure.
type Path string

// NewPath returns a new Path
func NewPath(items ...string) Path {
	return Path(strings.Join(items, pathSeparator))
}

// Next returns a new path by append part to the current path
func (p Path) Next(part string) Path {
	return Path(string(p) + pathSeparator + part)
}

func (p Path) parts() []string {
	return strings.Split(string(p), pathSeparator)
}

func (p Path) matches(pattern Path) bool {
	patternParts := pattern.parts()
	parts := p.parts()

	if len(patternParts) != len(parts) {
		return false
	}
	for index, part := range parts {
		switch patternParts[index] {
		case PathMatchAll, part:
			continue
		default:
			return false
		}
	}
	return true
}

func (o Options) getCasterForPath(path Path) (Cast, bool) {
	for pattern, caster := range o.TypeCastMapping {
		if path.matches(pattern) {
			return caster, true
		}
	}
	return nil, false
}
