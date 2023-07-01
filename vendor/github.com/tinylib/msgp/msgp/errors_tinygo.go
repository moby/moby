//go:build tinygo
// +build tinygo

package msgp

import (
	"reflect"
)

// ctxString converts the incoming interface{} slice into a single string,
// without using fmt under tinygo
func ctxString(ctx []interface{}) string {
	out := ""
	for idx, cv := range ctx {
		if idx > 0 {
			out += "/"
		}
		out += ifToStr(cv)
	}
	return out
}

type stringer interface {
	String() string
}

func ifToStr(i interface{}) string {
	switch v := i.(type) {
	case stringer:
		return v.String()
	case error:
		return v.Error()
	case string:
		return v
	default:
		return reflect.ValueOf(i).String()
	}
}

func quoteStr(s string) string {
	return simpleQuoteStr(s)
}
