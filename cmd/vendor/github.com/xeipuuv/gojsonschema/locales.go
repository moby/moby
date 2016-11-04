// Copyright 2015 xeipuuv ( https://github.com/xeipuuv )
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// author           xeipuuv
// author-github    https://github.com/xeipuuv
// author-mail      xeipuuv@gmail.com
//
// repository-name  gojsonschema
// repository-desc  An implementation of JSON Schema, based on IETF's draft v4 - Go language.
//
// description      Contains const string and messages.
//
// created          01-01-2015

package gojsonschema

type (
	// locale is an interface for definining custom error strings
	locale interface {
		Required() string
		InvalidType() string
		NumberAnyOf() string
		NumberOneOf() string
		NumberAllOf() string
		NumberNot() string
		MissingDependency() string
		Internal() string
		Enum() string
		ArrayNoAdditionalItems() string
		ArrayMinItems() string
		ArrayMaxItems() string
		Unique() string
		ArrayMinProperties() string
		ArrayMaxProperties() string
		AdditionalPropertyNotAllowed() string
		InvalidPropertyPattern() string
		StringGTE() string
		StringLTE() string
		DoesNotMatchPattern() string
		DoesNotMatchFormat() string
		MultipleOf() string
		NumberGTE() string
		NumberGT() string
		NumberLTE() string
		NumberLT() string

		// Schema validations
		RegexPattern() string
		GreaterThanZero() string
		MustBeOfA() string
		MustBeOfAn() string
		CannotBeUsedWithout() string
		CannotBeGT() string
		MustBeOfType() string
		MustBeValidRegex() string
		MustBeValidFormat() string
		MustBeGTEZero() string
		KeyCannotBeGreaterThan() string
		KeyItemsMustBeOfType() string
		KeyItemsMustBeUnique() string
		ReferenceMustBeCanonical() string
		NotAValidType() string
		Duplicated() string
		httpBadStatus() string

		// ErrorFormat
		ErrorFormat() string
	}

	// DefaultLocale is the default locale for this package
	DefaultLocale struct{}
)

func (l DefaultLocale) Required() string {
	return `%property% is required`
}

func (l DefaultLocale) InvalidType() string {
	return `Invalid type. Expected: %expected%, given: %given%`
}

func (l DefaultLocale) NumberAnyOf() string {
	return `Must validate at least one schema (anyOf)`
}

func (l DefaultLocale) NumberOneOf() string {
	return `Must validate one and only one schema (oneOf)`
}

func (l DefaultLocale) NumberAllOf() string {
	return `Must validate all the schemas (allOf)`
}

func (l DefaultLocale) NumberNot() string {
	return `Must not validate the schema (not)`
}

func (l DefaultLocale) MissingDependency() string {
	return `Has a dependency on %dependency%`
}

func (l DefaultLocale) Internal() string {
	return `Internal Error %error%`
}

func (l DefaultLocale) Enum() string {
	return `%field% must be one of the following: %allowed%`
}

func (l DefaultLocale) ArrayNoAdditionalItems() string {
	return `No additional items allowed on array`
}

func (l DefaultLocale) ArrayMinItems() string {
	return `Array must have at least %min% items`
}

func (l DefaultLocale) ArrayMaxItems() string {
	return `Array must have at most %max% items`
}

func (l DefaultLocale) Unique() string {
	return `%type% items must be unique`
}

func (l DefaultLocale) ArrayMinProperties() string {
	return `Must have at least %min% properties`
}

func (l DefaultLocale) ArrayMaxProperties() string {
	return `Must have at most %max% properties`
}

func (l DefaultLocale) AdditionalPropertyNotAllowed() string {
	return `Additional property %property% is not allowed`
}

func (l DefaultLocale) InvalidPropertyPattern() string {
	return `Property "%property%" does not match pattern %pattern%`
}

func (l DefaultLocale) StringGTE() string {
	return `String length must be greater than or equal to %min%`
}

func (l DefaultLocale) StringLTE() string {
	return `String length must be less than or equal to %max%`
}

func (l DefaultLocale) DoesNotMatchPattern() string {
	return `Does not match pattern '%pattern%'`
}

func (l DefaultLocale) DoesNotMatchFormat() string {
	return `Does not match format '%format%'`
}

func (l DefaultLocale) MultipleOf() string {
	return `Must be a multiple of %multiple%`
}

func (l DefaultLocale) NumberGTE() string {
	return `Must be greater than or equal to %min%`
}

func (l DefaultLocale) NumberGT() string {
	return `Must be greater than %min%`
}

func (l DefaultLocale) NumberLTE() string {
	return `Must be less than or equal to %max%`
}

func (l DefaultLocale) NumberLT() string {
	return `Must be less than %max%`
}

// Schema validators
func (l DefaultLocale) RegexPattern() string {
	return `Invalid regex pattern '%pattern%'`
}

func (l DefaultLocale) GreaterThanZero() string {
	return `%number% must be strictly greater than 0`
}

func (l DefaultLocale) MustBeOfA() string {
	return `%x% must be of a %y%`
}

func (l DefaultLocale) MustBeOfAn() string {
	return `%x% must be of an %y%`
}

func (l DefaultLocale) CannotBeUsedWithout() string {
	return `%x% cannot be used without %y%`
}

func (l DefaultLocale) CannotBeGT() string {
	return `%x% cannot be greater than %y%`
}

func (l DefaultLocale) MustBeOfType() string {
	return `%key% must be of type %type%`
}

func (l DefaultLocale) MustBeValidRegex() string {
	return `%key% must be a valid regex`
}

func (l DefaultLocale) MustBeValidFormat() string {
	return `%key% must be a valid format %given%`
}

func (l DefaultLocale) MustBeGTEZero() string {
	return `%key% must be greater than or equal to 0`
}

func (l DefaultLocale) KeyCannotBeGreaterThan() string {
	return `%key% cannot be greater than %y%`
}

func (l DefaultLocale) KeyItemsMustBeOfType() string {
	return `%key% items must be %type%`
}

func (l DefaultLocale) KeyItemsMustBeUnique() string {
	return `%key% items must be unique`
}

func (l DefaultLocale) ReferenceMustBeCanonical() string {
	return `Reference %reference% must be canonical`
}

func (l DefaultLocale) NotAValidType() string {
	return `%type% is not a valid type -- `
}

func (l DefaultLocale) Duplicated() string {
	return `%type% type is duplicated`
}

func (l DefaultLocale) httpBadStatus() string {
	return `Could not read schema from HTTP, response status is %status%`
}

// Replacement options: field, description, context, value
func (l DefaultLocale) ErrorFormat() string {
	return `%field%: %description%`
}

const (
	STRING_NUMBER                     = "number"
	STRING_ARRAY_OF_STRINGS           = "array of strings"
	STRING_ARRAY_OF_SCHEMAS           = "array of schemas"
	STRING_SCHEMA                     = "schema"
	STRING_SCHEMA_OR_ARRAY_OF_STRINGS = "schema or array of strings"
	STRING_PROPERTIES                 = "properties"
	STRING_DEPENDENCY                 = "dependency"
	STRING_PROPERTY                   = "property"
	STRING_UNDEFINED                  = "undefined"
	STRING_CONTEXT_ROOT               = "(root)"
	STRING_ROOT_SCHEMA_PROPERTY       = "(root)"
)
