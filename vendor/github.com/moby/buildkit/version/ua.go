package version

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var (
	reRelease *regexp.Regexp
	reDev     *regexp.Regexp
	reOnce    sync.Once
	uapCbs    map[string]func() string
)

func UserAgent() string {
	uaVersion := defaultVersion

	reOnce.Do(func() {
		reRelease = regexp.MustCompile(`^(v[0-9]+\.[0-9]+)\.[0-9]+$`)
		reDev = regexp.MustCompile(`^(v[0-9]+\.[0-9]+)\.[0-9]+`)
	})

	if matches := reRelease.FindAllStringSubmatch(Version, 1); len(matches) > 0 {
		uaVersion = matches[0][1]
	} else if matches := reDev.FindAllStringSubmatch(Version, 1); len(matches) > 0 {
		uaVersion = matches[0][1] + "-dev"
	}

	res := &strings.Builder{}
	fmt.Fprintf(res, "buildkit/%s", uaVersion)
	for pname, pver := range uapCbs {
		fmt.Fprintf(res, " %s/%s", pname, pver())
	}

	return res.String()
}

// SetUserAgentProduct sets a callback to get the version of a product to be
// included in the User-Agent header. The callback is called every time the
// User-Agent header is generated. Caller must ensure that the callback is
// cached if it is expensive to compute.
func SetUserAgentProduct(name string, cb func() (version string)) {
	if uapCbs == nil {
		uapCbs = make(map[string]func() string)
	}
	uapCbs[name] = cb
}
