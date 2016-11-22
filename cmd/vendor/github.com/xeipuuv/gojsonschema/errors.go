package gojsonschema

import (
	"fmt"
	"strings"
)

type (
	// RequiredError. ErrorDetails: property string
	RequiredError struct {
		ResultErrorFields
	}

	// InvalidTypeError. ErrorDetails: expected, given
	InvalidTypeError struct {
		ResultErrorFields
	}

	// NumberAnyOfError. ErrorDetails: -
	NumberAnyOfError struct {
		ResultErrorFields
	}

	// NumberOneOfError. ErrorDetails: -
	NumberOneOfError struct {
		ResultErrorFields
	}

	// NumberAllOfError. ErrorDetails: -
	NumberAllOfError struct {
		ResultErrorFields
	}

	// NumberNotError. ErrorDetails: -
	NumberNotError struct {
		ResultErrorFields
	}

	// MissingDependencyError. ErrorDetails: dependency
	MissingDependencyError struct {
		ResultErrorFields
	}

	// InternalError. ErrorDetails: error
	InternalError struct {
		ResultErrorFields
	}

	// EnumError. ErrorDetails: allowed
	EnumError struct {
		ResultErrorFields
	}

	// ArrayNoAdditionalItemsError. ErrorDetails: -
	ArrayNoAdditionalItemsError struct {
		ResultErrorFields
	}

	// ArrayMinItemsError. ErrorDetails: min
	ArrayMinItemsError struct {
		ResultErrorFields
	}

	// ArrayMaxItemsError. ErrorDetails: max
	ArrayMaxItemsError struct {
		ResultErrorFields
	}

	// ItemsMustBeUniqueError. ErrorDetails: type
	ItemsMustBeUniqueError struct {
		ResultErrorFields
	}

	// ArrayMinPropertiesError. ErrorDetails: min
	ArrayMinPropertiesError struct {
		ResultErrorFields
	}

	// ArrayMaxPropertiesError. ErrorDetails: max
	ArrayMaxPropertiesError struct {
		ResultErrorFields
	}

	// AdditionalPropertyNotAllowedError. ErrorDetails: property
	AdditionalPropertyNotAllowedError struct {
		ResultErrorFields
	}

	// InvalidPropertyPatternError. ErrorDetails: property, pattern
	InvalidPropertyPatternError struct {
		ResultErrorFields
	}

	// StringLengthGTEError. ErrorDetails: min
	StringLengthGTEError struct {
		ResultErrorFields
	}

	// StringLengthLTEError. ErrorDetails: max
	StringLengthLTEError struct {
		ResultErrorFields
	}

	// DoesNotMatchPatternError. ErrorDetails: pattern
	DoesNotMatchPatternError struct {
		ResultErrorFields
	}

	// DoesNotMatchFormatError. ErrorDetails: format
	DoesNotMatchFormatError struct {
		ResultErrorFields
	}

	// MultipleOfError. ErrorDetails: multiple
	MultipleOfError struct {
		ResultErrorFields
	}

	// NumberGTEError. ErrorDetails: min
	NumberGTEError struct {
		ResultErrorFields
	}

	// NumberGTError. ErrorDetails: min
	NumberGTError struct {
		ResultErrorFields
	}

	// NumberLTEError. ErrorDetails: max
	NumberLTEError struct {
		ResultErrorFields
	}

	// NumberLTError. ErrorDetails: max
	NumberLTError struct {
		ResultErrorFields
	}
)

// newError takes a ResultError type and sets the type, context, description, details, value, and field
func newError(err ResultError, context *jsonContext, value interface{}, locale locale, details ErrorDetails) {
	var t string
	var d string
	switch err.(type) {
	case *RequiredError:
		t = "required"
		d = locale.Required()
	case *InvalidTypeError:
		t = "invalid_type"
		d = locale.InvalidType()
	case *NumberAnyOfError:
		t = "number_any_of"
		d = locale.NumberAnyOf()
	case *NumberOneOfError:
		t = "number_one_of"
		d = locale.NumberOneOf()
	case *NumberAllOfError:
		t = "number_all_of"
		d = locale.NumberAllOf()
	case *NumberNotError:
		t = "number_not"
		d = locale.NumberNot()
	case *MissingDependencyError:
		t = "missing_dependency"
		d = locale.MissingDependency()
	case *InternalError:
		t = "internal"
		d = locale.Internal()
	case *EnumError:
		t = "enum"
		d = locale.Enum()
	case *ArrayNoAdditionalItemsError:
		t = "array_no_additional_items"
		d = locale.ArrayNoAdditionalItems()
	case *ArrayMinItemsError:
		t = "array_min_items"
		d = locale.ArrayMinItems()
	case *ArrayMaxItemsError:
		t = "array_max_items"
		d = locale.ArrayMaxItems()
	case *ItemsMustBeUniqueError:
		t = "unique"
		d = locale.Unique()
	case *ArrayMinPropertiesError:
		t = "array_min_properties"
		d = locale.ArrayMinProperties()
	case *ArrayMaxPropertiesError:
		t = "array_max_properties"
		d = locale.ArrayMaxProperties()
	case *AdditionalPropertyNotAllowedError:
		t = "additional_property_not_allowed"
		d = locale.AdditionalPropertyNotAllowed()
	case *InvalidPropertyPatternError:
		t = "invalid_property_pattern"
		d = locale.InvalidPropertyPattern()
	case *StringLengthGTEError:
		t = "string_gte"
		d = locale.StringGTE()
	case *StringLengthLTEError:
		t = "string_lte"
		d = locale.StringLTE()
	case *DoesNotMatchPatternError:
		t = "pattern"
		d = locale.DoesNotMatchPattern()
	case *DoesNotMatchFormatError:
		t = "format"
		d = locale.DoesNotMatchFormat()
	case *MultipleOfError:
		t = "multiple_of"
		d = locale.MultipleOf()
	case *NumberGTEError:
		t = "number_gte"
		d = locale.NumberGTE()
	case *NumberGTError:
		t = "number_gt"
		d = locale.NumberGT()
	case *NumberLTEError:
		t = "number_lte"
		d = locale.NumberLTE()
	case *NumberLTError:
		t = "number_lt"
		d = locale.NumberLT()
	}

	err.SetType(t)
	err.SetContext(context)
	err.SetValue(value)
	err.SetDetails(details)
	details["field"] = err.Field()
	err.SetDescription(formatErrorDescription(d, details))
}

// formatErrorDescription takes a string in this format: %field% is required
// and converts it to a string with replacements. The fields come from
// the ErrorDetails struct and vary for each type of error.
func formatErrorDescription(s string, details ErrorDetails) string {
	for name, val := range details {
		s = strings.Replace(s, "%"+strings.ToLower(name)+"%", fmt.Sprintf("%v", val), -1)
	}

	return s
}
