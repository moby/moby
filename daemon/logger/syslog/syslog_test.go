package syslog

import (
	"reflect"
	"testing"

	syslog "github.com/RackSec/srslog"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func functionMatches(expectedFun interface{}, actualFun interface{}) bool {
	return reflect.ValueOf(expectedFun).Pointer() == reflect.ValueOf(actualFun).Pointer()
}

func (s *DockerSuite) TestParseLogFormat(c *check.C) {
	formatter, framer, err := parseLogFormat("rfc5424", "udp")
	if err != nil || !functionMatches(rfc5424formatterWithAppNameAsTag, formatter) ||
		!functionMatches(syslog.DefaultFramer, framer) {
		c.Fatal("Failed to parse rfc5424 format", err, formatter, framer)
	}

	formatter, framer, err = parseLogFormat("rfc5424", "tcp+tls")
	if err != nil || !functionMatches(rfc5424formatterWithAppNameAsTag, formatter) ||
		!functionMatches(syslog.RFC5425MessageLengthFramer, framer) {
		c.Fatal("Failed to parse rfc5424 format", err, formatter, framer)
	}

	formatter, framer, err = parseLogFormat("rfc5424micro", "udp")
	if err != nil || !functionMatches(rfc5424microformatterWithAppNameAsTag, formatter) ||
		!functionMatches(syslog.DefaultFramer, framer) {
		c.Fatal("Failed to parse rfc5424 (microsecond) format", err, formatter, framer)
	}

	formatter, framer, err = parseLogFormat("rfc5424micro", "tcp+tls")
	if err != nil || !functionMatches(rfc5424microformatterWithAppNameAsTag, formatter) ||
		!functionMatches(syslog.RFC5425MessageLengthFramer, framer) {
		c.Fatal("Failed to parse rfc5424 (microsecond) format", err, formatter, framer)
	}

	formatter, framer, err = parseLogFormat("rfc3164", "")
	if err != nil || !functionMatches(syslog.RFC3164Formatter, formatter) ||
		!functionMatches(syslog.DefaultFramer, framer) {
		c.Fatal("Failed to parse rfc3164 format", err, formatter, framer)
	}

	formatter, framer, err = parseLogFormat("", "")
	if err != nil || !functionMatches(syslog.UnixFormatter, formatter) ||
		!functionMatches(syslog.DefaultFramer, framer) {
		c.Fatal("Failed to parse empty format", err, formatter, framer)
	}

	formatter, framer, err = parseLogFormat("invalid", "")
	if err == nil {
		c.Fatal("Failed to parse invalid format", err, formatter, framer)
	}
}

func (s *DockerSuite) TestValidateLogOptEmpty(c *check.C) {
	emptyConfig := make(map[string]string)
	if err := ValidateLogOpt(emptyConfig); err != nil {
		c.Fatal("Failed to parse empty config", err)
	}
}
