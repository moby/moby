// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/go-openapi/spec"
)

// minDistinctHostsToWarn is the number of distinct remote hosts among $refs at or above which
// Rule 2 emits a host-spread warning. A single consistent remote host is legitimate.
const minDistinctHostsToWarn = 2

// validateDubiousRefs emits warnings (never errors) when $ref locations match patterns
// that may indicate an unsafe or adversarial spec. It inspects refs as authored, on the
// UNEXPANDED spec, so it must run before expansion flattens them away.
//
// Two rules are applied over s.analyzer.AllRefs():
//
//   - Rule 1 (absolute local escape): a $ref pointing to an absolute local file location
//     (file:// scheme, a Unix absolute path, or a Windows drive path such as C:\) is dubious
//     UNLESS it stays beneath the spec's base path. Absolute refs beneath the base are
//     legitimate: flattening/expansion in go-openapi/spec and analysis introduces absolute
//     anchors to resolve cyclical $refs. Relative and fragment-only refs are always exempt.
//
//   - Rule 2 (host spread): when remote (http/https, or protocol-relative) refs resolve to
//     two or more distinct hosts, a single aggregate warning lists them. A single consistent
//     remote host is common and legitimate, so it is not flagged.
//
// All findings are warnings: they do not affect validity (see Result.IsValid).
func (s *SpecValidator) validateDubiousRefs() *Result {
	res := pools.poolOfResults.BorrowResult()

	baseDir, hasBase := s.localBaseDir()

	remoteHosts := make(map[string]struct{})
	for _, r := range s.analyzer.AllRefs() {
		u := r.GetURL()
		if u == nil { // Safeguard: a valid spec always yields parseable refs
			continue
		}

		// Rule 1: absolute local reference escaping the base path.
		if refPath, isLocalAbs := absoluteLocalRefPath(r, u); isLocalAbs {
			if !hasBase || !isBeneathBase(refPath, baseDir) {
				res.AddWarnings(dubiousAbsoluteRefMsg(r.String()))
			}
			continue
		}

		// Rule 2: gather remote hosts (http/https and protocol-relative //host/...).
		if host := remoteRefHost(u); host != "" {
			remoteHosts[host] = struct{}{}
		}
	}

	if len(remoteHosts) >= minDistinctHostsToWarn {
		hosts := make([]string, 0, len(remoteHosts))
		for h := range remoteHosts {
			hosts = append(hosts, h)
		}
		sort.Strings(hosts)
		res.AddWarnings(dubiousMultipleHostsMsg(len(hosts), strings.Join(hosts, ", ")))
	}

	return res
}

// absoluteLocalRefPath reports whether r is an absolute LOCAL file reference and, if so,
// returns the cleaned path it points to (without scheme/fragment, drive letter lower-cased).
//
// Classification order matters (see the empirical jsonreference flag behavior):
//   - file:// scheme is local, including UNC file://host/share (inherently dubious).
//   - a non-empty Host with no file scheme means remote (http/https or protocol-relative
//     //host/path) - NOT local; handled by Rule 2. This must be checked before the Unix
//     branch, because protocol-relative refs also set HasFullFilePath.
//   - len(u.Scheme) == 1 is a Windows drive path (C:\ or C:/), whose drive+path land in
//     Scheme/Opaque/Path rather than Path. Checked before the Unix branch because C:/x also
//     sets HasFullFilePath, and reconstructed from the authored ref string to keep the drive.
//   - !r.HasFullURL && r.HasFullFilePath is a plain Unix absolute path (/abs/models.json).
//
// Relative (./x.json) and fragment-only (#/definitions/X) refs return false.
func absoluteLocalRefPath(r spec.Ref, u *url.URL) (string, bool) {
	switch {
	case r.HasFileScheme:
		return fileRefPath(u), true
	case u.Host != "":
		// Remote (http/https) or protocol-relative //host/path: handled by Rule 2.
		return "", false
	case len(u.Scheme) == 1:
		// Windows drive letter: reconstruct from the authored ref string.
		return cleanRefPath(r.String()), true
	case !r.HasFullURL && r.HasFullFilePath:
		return cleanRefPath(u.Path), true
	default:
		return "", false
	}
}

// remoteRefHost returns the host of a remote reference (http/https), or of a protocol-relative
// reference (//host/path). It returns "" for local and fragment-only refs. file:// hosts (UNC)
// are deliberately excluded: those are handled as local-absolute refs by Rule 1.
func remoteRefHost(u *url.URL) string {
	switch u.Scheme {
	case "http", "https":
		return u.Host
	case "":
		// Protocol-relative //host/path: empty scheme but a host is present.
		return u.Host
	default:
		return ""
	}
}

// localBaseDir returns the directory of the spec file, slash-normalized, when the spec was
// loaded from a local path. It returns ok=false when the base is unknown (in-memory spec) or
// remote (http/https), in which case absolute-local refs cannot be proven beneath a base and
// are treated as dubious.
func (s *SpecValidator) localBaseDir() (string, bool) {
	specPath := s.spec.SpecFilePath()
	if specPath == "" {
		return "", false
	}

	// Strip a file:// scheme if present; reject remote bases.
	if u, err := url.Parse(specPath); err == nil && u.Scheme != "" {
		switch {
		case u.Scheme == "file":
			specPath = u.Path
		case len(u.Scheme) == 1: // Windows drive letter, treat as local
			// keep specPath as-is (authored path)
		default: // http, https, ... : no local base
			return "", false
		}
	}

	return path.Dir(cleanRefPath(specPath)), true
}

// isBeneathBase reports whether the cleaned target path is located within baseDir, i.e. it does
// not escape baseDir via "..". Comparison is purely lexical on cleaned paths, which is sufficient
// (and cross-platform safe) for a non-fatal warning. Both sides are expected to already be
// cleanRefPath-normalized (slashes, drive-letter case).
func isBeneathBase(target, baseDir string) bool {
	if baseDir == "" {
		return false
	}
	if target == baseDir {
		return true
	}
	if !strings.HasSuffix(baseDir, "/") {
		baseDir += "/"
	}
	return strings.HasPrefix(target, baseDir)
}

// fileRefPath extracts the local path a file:// reference points to, accounting for the way
// Windows file URLs parse:
//   - file:///abs/x     -> /abs/x           (empty host)
//   - file:///C:/dir/x  -> /c:/dir/x        (empty host; drive sits in the path)
//   - file://D:/a/x     -> d:/a/x           (drive letter lands in Host, rejoin it)
//   - file://host/share -> /host/share/x    (real UNC host kept visible so it cannot match a
//     local base and stays flagged as dubious)
func fileRefPath(u *url.URL) string {
	switch {
	case u.Host == "":
		return cleanRefPath(u.Path)
	case isDriveHost(u.Host):
		// Windows path authored as file://D:/... : the drive landed in Host (e.g. "d:").
		return cleanRefPath(u.Host + u.Path)
	default:
		// Real remote/UNC host: keep it in the path so it never matches a local base.
		return cleanRefPath("//" + u.Host + u.Path)
	}
}

// isDriveHost reports whether a URL host is actually a Windows drive letter (e.g. "d:"), which
// happens when a Windows path is authored as a two-slash file URL: file://D:/path.
func isDriveHost(host string) bool {
	h := strings.TrimSuffix(host, ":")
	if len(h) != 1 {
		return false
	}
	c := h[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// cleanRefPath normalizes a ref or base path for lexical comparison: backslashes to forward
// slashes, path.Clean, and a lower-cased leading Windows drive letter (matching the behavior of
// go-openapi/spec's normalizer). Plain Unix paths are unaffected, preserving case-sensitivity.
func cleanRefPath(p string) string {
	p = path.Clean(strings.ReplaceAll(p, `\`, `/`))
	switch {
	case len(p) >= 2 && p[1] == ':':
		// drive-letter form: C:/dir -> c:/dir
		p = strings.ToLower(p[:1]) + p[1:]
	case len(p) >= 3 && p[0] == '/' && p[2] == ':':
		// slash-prefixed drive form from canonical file:// URLs: /C:/dir -> c:/dir.
		// The leading slash is dropped so this matches the base path derived from
		// SpecFilePath (which has no leading slash), and the bare-drive form.
		p = strings.ToLower(p[1:2]) + p[2:]
	}
	return p
}
