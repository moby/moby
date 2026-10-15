package errors

import (
	"reflect"
	"strings"

	"github.com/aws/smithy-go"
)

// SetErrorCodeOverride sets the ErrorCodeOverride field on err if present.
//
// This is used by protocols in awsquery-compatible mode that need to override
// the error code on a deserialized error.
func SetErrorCodeOverride(err error, code string) {
	// yes it's reflection but i don't really view errors as a hot path so i
	// think it's fine
	v := reflect.ValueOf(err)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}

	f := v.FieldByName("ErrorCodeOverride")
	if !f.IsValid() || !f.CanSet() || f.Type() != reflect.TypeOf((*string)(nil)) {
		return
	}

	f.Set(reflect.ValueOf(&code))
}

// SanitizeErrorCode strips namespace prefixes and colon suffixes from protocol
// error codes.
func SanitizeErrorCode(code string) string {
	_, noprefix, ok := strings.Cut(code, "#")
	if !ok {
		noprefix = code // If sep does not appear in s, cut returns s, "", false.
	}

	code, _, _ = strings.Cut(noprefix, ":")
	return code
}

// ParseQueryError extracts the error code and fault from X-Amzn-Query-Error.
func ParseQueryError(header string) (string, smithy.ErrorFault) {
	code, fault, _ := strings.Cut(header, ";")
	switch fault {
	case "Sender":
		return code, smithy.FaultClient
	case "Receiver":
		return code, smithy.FaultServer
	default:
		return code, smithy.FaultUnknown
	}
}
