package sys

import "math"

// UTIME_OMIT is a special constant for use in updating times via FS.Utimens
// or File.Utimens. When used for atim or mtim, the value is retained.
//
// Note: This may be implemented via a stat when the underlying filesystem
// does not support this value.
const UTIME_OMIT int64 = math.MinInt64
