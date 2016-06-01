package srslog

import (
	"fmt"
)

// Framer is a type of function that takes an input string (typically an
// already-formatted syslog message) and applies "message framing" to it. We
// have different framers because different versions of the syslog protocol
// and its transport requirements define different framing behavior.
type Framer func(in string) string

// DefaultFramer does nothing, since there is no framing to apply. This is
// the original behavior of the Go syslog package, and is also typically used
// for UDP syslog.
func DefaultFramer(in string) string {
	return in
}

// RFC5425MessageLengthFramer prepends the message length to the front of the
// provided message, as defined in RFC 5425.
func RFC5425MessageLengthFramer(in string) string {
	return fmt.Sprintf("%d %s", len(in), in)
}
