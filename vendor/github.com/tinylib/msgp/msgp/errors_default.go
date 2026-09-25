//go:build !tinygo
// +build !tinygo

package msgp

import (
	"fmt"
	"strconv"
)

// ctxString converts the incoming interface{} slice into a single string.
func ctxString(ctx []interface{}) string {
	out := ""
	for idx, cv := range ctx {
		if idx > 0 {
			out += "/"
		}
		out += fmt.Sprintf("%v", cv)
	}
	return out
}

func quoteStr(s string) string {
	return strconv.Quote(s)
}
