package escapetest

// This file is part of escape_json_string unit test.
// It is in a separate package so cgo can be used together
// with go test.

// #include <stdlib.h>
// extern char *escape_json_string(char *str);
// #cgo CFLAGS: -DESCAPE_TEST=1
import "C"

import (
	"testing"
	"unsafe"
)

func testEscapeJSONString(t *testing.T, input, want string) {
	in := C.CString(input)
	out := C.escape_json_string(in)
	got := C.GoString(out)
	C.free(unsafe.Pointer(out))
	t.Logf("input: %q, output: %q", input, got)
	if got != want {
		t.Errorf("Failed on input: %q, want %q, got %q", input, want, got)
	}
}

func testEscapeJSON(t *testing.T) {
	testCases := []struct {
		input, output string
	}{
		{"", ""},
		{"abcdef", "abcdef"},
		{`\\\\\\`, `\\\\\\\\\\\\`},
		{`with"quote`, `with\"quote`},
		{"\n\r\b\t\f\\", `\n\r\b\t\f\\`},
		{"\007", "\\u0007"},
		{"\017 \020 \037", "\\u000f \\u0010 \\u001f"},
		{"\033", "\\u001b"},
		{`<->`, `<->`},
		{"\176\177\200", "~\\u007f\200"},
		{"\000", ""},
		{"a\x7fxc", "a\\u007fxc"},
		{"a\033xc", "a\\u001bxc"},
		{"a\nxc", "a\\nxc"},
		{"a\\xc", "a\\\\xc"},
		{"Barney B\303\244r", "Barney B\303\244r"},
	}

	for _, tc := range testCases {
		testEscapeJSONString(t, tc.input, tc.output)
	}
}
