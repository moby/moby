package dns

import (
	"regexp"
)

const IpLocalhost = `((127\.([0-9]{1,3}.){2}[0-9]{1,3})|(::1))`

var localhostIPRegexp = regexp.MustCompile(IpLocalhost)

// IsLocalhost returns true if ip matches the localhost IP regular expression.
// Used for determining if nameserver settings are being passed which are
// localhost addresses
func IsLocalhost(ip string) bool {
	return localhostIPRegexp.MatchString(ip)
}
