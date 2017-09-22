package jsonlog

import (
	"regexp"
	"testing"

	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONLogMarshalJSON(t *testing.T) {
	logs := map[*JSONLog]string{
		{Log: `"A log line with \\"`}:           `^{\"log\":\"\\\"A log line with \\\\\\\\\\\"\",\"time\":\".{20,}\"}$`,
		{Log: "A log line"}:                     `^{\"log\":\"A log line\",\"time\":\".{20,}\"}$`,
		{Log: "A log line with \r"}:             `^{\"log\":\"A log line with \\r\",\"time\":\".{20,}\"}$`,
		{Log: "A log line with & < >"}:          `^{\"log\":\"A log line with \\u0026 \\u003c \\u003e\",\"time\":\".{20,}\"}$`,
		{Log: "A log line with utf8 : 🚀 ψ ω β"}: `^{\"log\":\"A log line with utf8 : 🚀 ψ ω β\",\"time\":\".{20,}\"}$`,
		{Stream: "stdout"}:                      `^{\"stream\":\"stdout\",\"time\":\".{20,}\"}$`,
		{}:                                      `^{\"time\":\".{20,}\"}$`,
		// These ones are a little weird
		{Log: "\u2028 \u2029"}:      `^{\"log\":\"\\u2028 \\u2029\",\"time\":\".{20,}\"}$`,
		{Log: string([]byte{0xaF})}: `^{\"log\":\"\\ufffd\",\"time\":\".{20,}\"}$`,
		{Log: string([]byte{0x7F})}: `^{\"log\":\"\x7f\",\"time\":\".{20,}\"}$`,
	}
	for jsonLog, expression := range logs {
		data, err := jsonLog.MarshalJSON()
		require.NoError(t, err)
		assert.Regexp(t, regexp.MustCompile(expression), string(data))
		assert.NoError(t, json.Unmarshal(data, &map[string]interface{}{}))
	}
}
