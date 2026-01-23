package apple

import "time"

// In the context of a root policy update on trusted certificate lifetimes[0]
// Apple provided an unambiguous definition for the length of a day:
//
//	"398 days is measured with a day being equal to 86,400 seconds. Any time
//	greater than this indicates an additional day of validity."
//
// We provide that value as a constant here for lints to use.
//
// [0]: https://support.apple.com/en-us/HT211025
var appleDayLength = 86400 * time.Second
