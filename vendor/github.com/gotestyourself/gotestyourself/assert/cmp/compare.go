/*Package cmp provides Comparisons for Assert and Check*/
package cmp

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/pmezard/go-difflib/difflib"
)

// Compare two complex values using https://godoc.org/github.com/google/go-cmp/cmp
// and succeeds if the values are equal.
//
// The comparison can be customized using comparison Options.
func Compare(x, y interface{}, opts ...cmp.Option) func() (bool, string) {
	return func() (bool, string) {
		diff := cmp.Diff(x, y, opts...)
		return diff == "", "\n" + diff
	}
}

// Equal succeeds if x == y.
func Equal(x, y interface{}) func() (success bool, message string) {
	return func() (bool, string) {
		return x == y, fmt.Sprintf("%v (%T) != %v (%T)", x, x, y, y)
	}
}

// Len succeeds if the sequence has the expected length.
func Len(seq interface{}, expected int) func() (bool, string) {
	return func() (success bool, message string) {
		defer func() {
			if e := recover(); e != nil {
				success = false
				message = fmt.Sprintf("type %T does not have a length", seq)
			}
		}()
		value := reflect.ValueOf(seq)
		length := value.Len()
		if length == expected {
			return true, ""
		}
		msg := fmt.Sprintf("expected %s (length %d) to have length %d", seq, length, expected)
		return false, msg
	}
}

// NilError succeeds if the last argument is a nil error.
func NilError(arg interface{}, args ...interface{}) func() (bool, string) {
	return func() (bool, string) {
		msgFunc := func(value reflect.Value) string {
			return fmt.Sprintf("error is not nil: %s", value.Interface().(error).Error())
		}
		if len(args) == 0 {
			return isNil(arg, msgFunc)()
		}
		return isNil(args[len(args)-1], msgFunc)()
	}
}

// Contains succeeds if item is in collection. Collection may be a string, map,
// slice, or array.
//
// If collection is a string, item must also be a string, and is compared using
// strings.Contains().
// If collection is a Map, contains will succeed if item is a key in the map.
// If collection is a slice or array, item is compared to each item in the
// sequence using reflect.DeepEqual().
func Contains(collection interface{}, item interface{}) func() (bool, string) {
	return func() (bool, string) {
		colValue := reflect.ValueOf(collection)
		if !colValue.IsValid() {
			return false, fmt.Sprintf("nil does not contain items")
		}
		msg := fmt.Sprintf("%v does not contain %v", collection, item)

		itemValue := reflect.ValueOf(item)
		switch colValue.Type().Kind() {
		case reflect.String:
			if itemValue.Type().Kind() != reflect.String {
				return false, "string may only contain strings"
			}
			success := strings.Contains(colValue.String(), itemValue.String())
			return success, fmt.Sprintf("string %q does not contain %q", collection, item)

		case reflect.Map:
			if itemValue.Type() != colValue.Type().Key() {
				return false, fmt.Sprintf(
					"%v can not contain a %v key", colValue.Type(), itemValue.Type())
			}
			index := colValue.MapIndex(itemValue)
			return index.IsValid(), msg

		case reflect.Slice, reflect.Array:
			for i := 0; i < colValue.Len(); i++ {
				if reflect.DeepEqual(colValue.Index(i).Interface(), item) {
					return true, ""
				}
			}
			return false, msg
		default:
			return false, fmt.Sprintf("type %T does not contain items", collection)
		}
	}
}

// Panics succeeds if f() panics.
func Panics(f func()) func() (bool, string) {
	return func() (success bool, message string) {
		defer func() {
			if err := recover(); err != nil {
				success = true
			}
		}()
		f()
		return false, "did not panic"
	}
}

// EqualMultiLine succeeds if the two strings are equal. If they are not equal
// the failure message will be the difference between the two strings.
func EqualMultiLine(x, y string) func() (bool, string) {
	return func() (bool, string) {
		if x == y {
			return true, ""
		}

		diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(x),
			B:        difflib.SplitLines(y),
			FromFile: "left",
			ToFile:   "right",
			Context:  3,
		})
		if err != nil {
			return false, fmt.Sprintf("failed to produce diff: %s", err)
		}
		return false, "\n" + diff
	}
}

// Error succeeds if err is a non-nil error, and the error message equals the
// expected message.
func Error(err error, message string) func() (bool, string) {
	return func() (bool, string) {
		switch {
		case err == nil:
			return false, "expected an error, got nil"
		case err.Error() != message:
			return false, fmt.Sprintf(
				"expected error message %q, got %q", message, err.Error())
		}
		return true, ""
	}
}

// ErrorContains succeeds if err is a non-nil error, and the error message contains
// the expected substring.
func ErrorContains(err error, substring string) func() (bool, string) {
	return func() (bool, string) {
		switch {
		case err == nil:
			return false, "expected an error, got nil"
		case !strings.Contains(err.Error(), substring):
			return false, fmt.Sprintf(
				"expected error message to contain %q, got %q", substring, err.Error())
		}
		return true, ""
	}
}

// Nil succeeds if obj is a nil interface, pointer, or function.
//
// Use NilError() for comparing errors. Use Len(obj, 0) for comparing slices,
// maps, and channels.
func Nil(obj interface{}) func() (bool, string) {
	msgFunc := func(value reflect.Value) string {
		return fmt.Sprintf("%v (type %s) is not nil", reflect.Indirect(value), value.Type())
	}
	return isNil(obj, msgFunc)
}

func isNil(obj interface{}, msgFunc func(reflect.Value) string) func() (bool, string) {
	return func() (bool, string) {
		if obj == nil {
			return true, ""
		}
		value := reflect.ValueOf(obj)
		kind := value.Type().Kind()
		if kind >= reflect.Chan && kind <= reflect.Slice {
			if value.IsNil() {
				return true, ""
			}
			return false, msgFunc(value)
		}

		return false, fmt.Sprintf("%v (type %s) can not be nil", value, value.Type())
	}
}
