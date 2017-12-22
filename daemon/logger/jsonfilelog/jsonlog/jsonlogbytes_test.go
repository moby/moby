package jsonlog // import "github.com/docker/docker/daemon/logger/jsonfilelog/jsonlog"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/gotestyourself/gotestyourself/assert"
)

func TestJSONLogsMarshalJSONBuf(t *testing.T) {
	logs := map[*JSONLogs]string{
		{Log: []byte(`"A log line with \\"`)}:                  `^{\"log\":\"\\\"A log line with \\\\\\\\\\\"\",\"time\":`,
		{Log: []byte("A log line")}:                            `^{\"log\":\"A log line\",\"time\":`,
		{Log: []byte("A log line with \r")}:                    `^{\"log\":\"A log line with \\r\",\"time\":`,
		{Log: []byte("A log line with & < >")}:                 `^{\"log\":\"A log line with \\u0026 \\u003c \\u003e\",\"time\":`,
		{Log: []byte("A log line with utf8 : 🚀 ψ ω β")}:        `^{\"log\":\"A log line with utf8 : 🚀 ψ ω β\",\"time\":`,
		{Stream: "stdout"}:                                     `^{\"stream\":\"stdout\",\"time\":`,
		{Stream: "stdout", Log: []byte("A log line")}:          `^{\"log\":\"A log line\",\"stream\":\"stdout\",\"time\":`,
		{Created: time.Date(2017, 9, 1, 1, 1, 1, 1, time.UTC)}: `^{\"time\":"2017-09-01T01:01:01.000000001Z"}$`,

		{}: `^{\"time\":"0001-01-01T00:00:00Z"}$`,
		// These ones are a little weird
		{Log: []byte("\u2028 \u2029")}: `^{\"log\":\"\\u2028 \\u2029\",\"time\":`,
		{Log: []byte{0xaF}}:            `^{\"log\":\"\\ufffd\",\"time\":`,
		{Log: []byte{0x7F}}:            `^{\"log\":\"\x7f\",\"time\":`,
		// with raw attributes
		{Log: []byte("A log line"), RawAttrs: []byte(`{"hello":"world","value":1234}`)}: `^{\"log\":\"A log line\",\"attrs\":{\"hello\":\"world\",\"value\":1234},\"time\":`,
		// with Tag set
		{Log: []byte("A log line with tag"), RawAttrs: []byte(`{"hello":"world","value":1234}`)}: `^{\"log\":\"A log line with tag\",\"attrs\":{\"hello\":\"world\",\"value\":1234},\"time\":`,
	}
	for jsonLog, expression := range logs {
		var buf bytes.Buffer
		err := jsonLog.MarshalJSONBuf(&buf)
		assert.NilError(t, err)

		assert.Assert(t, regexP(buf.String(), expression))
		assert.NilError(t, json.Unmarshal(buf.Bytes(), &map[string]interface{}{}))
	}
}

func regexP(value string, pattern string) func() (bool, string) {
	return func() (bool, string) {
		re := regexp.MustCompile(pattern)
		msg := fmt.Sprintf("%q did not match pattern %q", value, pattern)
		return re.MatchString(value), msg
	}
}
