// +build linux

package syslog

import (
	syslog "github.com/RackSec/srslog"
	"reflect"
	"testing"
)

func functionMatches(expectedFun interface{}, actualFun interface{}) bool {
	return reflect.ValueOf(expectedFun).Pointer() == reflect.ValueOf(actualFun).Pointer()
}

func TestParseLogFormat(t *testing.T) {
	formatter, framer, err := parseLogFormat("rfc5424")
	if err != nil || !functionMatches(rfc5424formatterWithAppNameAsTag, formatter) ||
		!functionMatches(syslog.RFC5425MessageLengthFramer, framer) {
		t.Fatal("Failed to parse rfc5424 format", err, formatter, framer)
	}

	formatter, framer, err = parseLogFormat("rfc5424micro")
	if err != nil || !functionMatches(rfc5424microformatterWithAppNameAsTag, formatter) ||
		!functionMatches(syslog.RFC5425MessageLengthFramer, framer) {
		t.Fatal("Failed to parse rfc5424 (microsecond) format", err, formatter, framer)
	}

	formatter, framer, err = parseLogFormat("rfc3164")
	if err != nil || !functionMatches(syslog.RFC3164Formatter, formatter) ||
		!functionMatches(syslog.DefaultFramer, framer) {
		t.Fatal("Failed to parse rfc3164 format", err, formatter, framer)
	}

	formatter, framer, err = parseLogFormat("")
	if err != nil || !functionMatches(syslog.UnixFormatter, formatter) ||
		!functionMatches(syslog.DefaultFramer, framer) {
		t.Fatal("Failed to parse empty format", err, formatter, framer)
	}

	formatter, framer, err = parseLogFormat("invalid")
	if err == nil {
		t.Fatal("Failed to parse invalid format", err, formatter, framer)
	}
}

func TestValidateLogOptEmpty(t *testing.T) {
	emptyConfig := make(map[string]string)
	if err := ValidateLogOpt(emptyConfig); err != nil {
		t.Fatal("Failed to parse empty config", err)
	}
}
