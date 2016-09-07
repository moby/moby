package jsonlog

import (
	"regexp"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestJSONLogMarshalJSON(c *check.C) {
	logs := map[*JSONLog]string{
		&JSONLog{Log: `"A log line with \\"`}:           `^{\"log\":\"\\\"A log line with \\\\\\\\\\\"\",\"time\":\".{20,}\"}$`,
		&JSONLog{Log: "A log line"}:                     `^{\"log\":\"A log line\",\"time\":\".{20,}\"}$`,
		&JSONLog{Log: "A log line with \r"}:             `^{\"log\":\"A log line with \\r\",\"time\":\".{20,}\"}$`,
		&JSONLog{Log: "A log line with & < >"}:          `^{\"log\":\"A log line with \\u0026 \\u003c \\u003e\",\"time\":\".{20,}\"}$`,
		&JSONLog{Log: "A log line with utf8 : 🚀 ψ ω β"}: `^{\"log\":\"A log line with utf8 : 🚀 ψ ω β\",\"time\":\".{20,}\"}$`,
		&JSONLog{Stream: "stdout"}:                      `^{\"stream\":\"stdout\",\"time\":\".{20,}\"}$`,
		&JSONLog{}:                                      `^{\"time\":\".{20,}\"}$`,
		// These ones are a little weird
		&JSONLog{Log: "\u2028 \u2029"}:      `^{\"log\":\"\\u2028 \\u2029\",\"time\":\".{20,}\"}$`,
		&JSONLog{Log: string([]byte{0xaF})}: `^{\"log\":\"\\ufffd\",\"time\":\".{20,}\"}$`,
		&JSONLog{Log: string([]byte{0x7F})}: `^{\"log\":\"\x7f\",\"time\":\".{20,}\"}$`,
	}
	for jsonLog, expression := range logs {
		data, err := jsonLog.MarshalJSON()
		if err != nil {
			c.Fatal(err)
		}
		res := string(data)
		c.Logf("Result of WriteLog: %q", res)
		logRe := regexp.MustCompile(expression)
		if !logRe.MatchString(res) {
			c.Fatalf("Log line not in expected format [%v]: %q", expression, res)
		}
	}
}
