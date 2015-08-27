// +build windows

package opts

import "fmt"

// DefaultHost constant defines the default host string used by docker on Windows
var DefaultHost = fmt.Sprintf("tcp://%s:%d", DefaultHTTPHost, DefaultHTTPPort)
