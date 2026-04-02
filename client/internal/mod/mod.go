// Package mod provides a small helper to extract a module's version
// from [debug.BuildInfo] without depending on [golang.org/x/mod].
//
// [golang.org/x/mod]: https://pkg.go.dev/golang.org/x/mod
package mod

import (
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
)

var readBuildInfo = sync.OnceValues(debug.ReadBuildInfo)

// Version returns a best-effort version string for the given module path,
// similar to [mod.Version] in the daemon.
//
// If the module is present in [debug.BuildInfo] dependencies, its version
// is returned. Tagged versions are returned as-is (with "+incompatible"
// stripped). [Pseudo-versions] are normalized to:
//
//	<base>+<revision>[+meta...][+dirty]
//
// Where "<base>" matches the behavior of [module.PseudoVersionBase] (i.e.,
// downgrade to the previous tag for non-prerelease Pseudo-versions).
//
// If the module is replaced (for example via go.work or replace directives),
// or no usable version information is available, Version returns an empty string.
//
// The returned value is intended for display purposes (e.g., in a default
// User-Agent), not for version comparison.
//
// [mod.Version]: https://pkg.go.dev/github.com/moby/moby/v2@v2.0.0-beta.7/daemon/internal/builder-next/worker/mod#Version
// [module.PseudoVersionBase]: https://pkg.go.dev/golang.org/x/mod@v0.34.0/module#PseudoVersionBase
// [Pseudo-versions]: https://cs.opensource.google/go/x/mod/+/refs/tags/v0.34.0:module/pseudo.go;l=5-33
func Version(name string) string {
	bi, ok := readBuildInfo()
	if !ok || bi == nil {
		return ""
	}
	return moduleVersion(name, bi)
}

func moduleVersion(name string, bi *debug.BuildInfo) (modVersion string) {
	if bi == nil {
		return ""
	}

	// Check if we're the main module.
	if v, ok := getVersion(name, &bi.Main); ok {
		return v
	}

	// iterate over all dependencies and find name
	for _, dep := range bi.Deps {
		if v, ok := getVersion(name, dep); ok {
			return v
		}
	}

	return ""
}

func getVersion(name string, dep *debug.Module) (string, bool) {
	if dep == nil || dep.Path != name {
		return "", false
	}

	v := dep.Version
	if dep.Replace != nil && dep.Replace.Version != "" {
		v = dep.Replace.Version
	}
	if v == "" || v == "(devel)" {
		return "", true
	}

	return normalize(v), true
}

// normalize converts a Go module version into a display-friendly form:
//
//   - strips "+incompatible" unconditionally
//   - if pseudo: vX.Y.Z[-pre][+rev][+meta...][+dirty]
//   - if tagged: vX.Y.Z[-pre][+meta...][+dirty]
func normalize(v string) string {
	base, metas, dirty := splitMetadata(v)

	out := base
	if base2, rev, undoPatch, ok := splitPseudo(base); ok {
		if undoPatch {
			// Downgrade the patch version that was raised by pseudo-versions:
			//
			//	(2) vX.Y.(Z+1)-0.yyyymmddhhmmss-abcdef123456
			if major, minor, patch, ok := parseSemVer(base2); ok && patch > 0 {
				patch--
				base2 = fmt.Sprintf("v%d.%d.%d", major, minor, patch)
			}
		}
		// Go pseudo rev is typically 12, but be defensive.
		if len(rev) > 12 {
			rev = rev[:12]
		}
		out = base2 + "+" + rev
	}

	// Preserve other metadata (except for "+incompatible").
	for _, m := range metas {
		out += m
	}
	if dirty {
		// +dirty goes last
		out += "+dirty"
	}
	return out
}

func splitMetadata(v string) (base string, metas []string, dirty bool) {
	base, meta, ok := strings.Cut(v, "+")
	if !ok || meta == "" {
		return base, nil, false
	}
	for m := range strings.SplitSeq(meta, "+") {
		// drop incompatible, extract dirty, preserve everything else.
		switch m {
		case "incompatible", "":
			// drop "+incompatible" and empty strings
		case "dirty":
			dirty = true
		default:
			metas = append(metas, "+"+m)
		}
	}

	return base, metas, dirty
}

// splitPseudo splits a pseudo-version into base + revision, and reports whether
// it is a (Z+1) pseudo that needs patch undo.
//
// Supported (after stripping +incompatible/+dirty metadata):
//
// (1) vX.0.0-yyyymmddhhmmss-abcdef123456
// (2) vX.Y.(Z+1)-0.yyyymmddhhmmss-abcdef123456
// (4) vX.Y.Z-pre.0.yyyymmddhhmmss-abcdef123456
func splitPseudo(v string) (base, rev string, undoPatch bool, ok bool) {
	// Split off revision at the last '-'.
	last := strings.LastIndexByte(v, '-')
	if last < 0 || last+1 >= len(v) {
		return "", "", false, false
	}
	rev = v[last+1:]
	left := v[:last]

	// First try the dot-joined timestamp forms:
	//   ...-0.<ts>   (release pseudo; undoPatch)
	//   ....0.<ts>   (prerelease pseudo; preserve prerelease)
	if dot := strings.LastIndexByte(left, '.'); dot > 0 && dot+1 < len(left) {
		ts := left[dot+1:]
		if isTimestamp(ts) {
			prefix := left[:dot] // ends with "-0" or ".0" for forms (2)/(4)
			switch {
			case strings.HasSuffix(prefix, "-0"):
				// (2) vX.Y.(Z+1)-0.yyyymmddhhmmss-abcdef123456
				return prefix[:len(prefix)-2], rev, true, true
			case strings.HasSuffix(prefix, ".0"):
				// (4) vX.Y.Z-pre.0.yyyymmddhhmmss-abcdef123456
				return prefix[:len(prefix)-2], rev, false, true
			}
		}
	}

	// Fall back to form (1): ...-<ts>-<rev>
	//
	// (1) vX.0.0-yyyymmddhhmmss-abcdef123456
	if dash := strings.LastIndexByte(left, '-'); dash > 0 && dash+1 < len(left) {
		ts := left[dash+1:]
		if isTimestamp(ts) {
			return left[:dash], rev, false, true
		}
	}

	return "", "", false, false
}

// isTimestamp checks whether s is a timestamp ("yyyymmddhhmmss")
// component in a module version (vX.0.0-yyyymmddhhmmss-abcdef123456).
func isTimestamp(s string) bool {
	if len(s) != 14 {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// parseSemVer parses "vX.Y.Z" into numeric components.
// It intentionally handles only the strict three-segment core form.
func parseSemVer(v string) (major, minor, patch int, ok bool) {
	if len(v) < 2 || v[0] != 'v' {
		return 0, 0, 0, false
	}
	parts := strings.Split(v[1:], ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	var err error
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, false
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, false
	}
	patch, err = strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, false
	}
	return major, minor, patch, true
}
