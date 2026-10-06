// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"net/url"
	"path"
	"strings"
)

const fileScheme = "file"

// normalizeURI ensures that all $ref paths used internally by the expander are canonicalized.
//
// NOTE(windows): there is a tolerance over the strict URI format on windows.
//
// The normalizer accepts relative file URLs like 'Path\File.JSON' as well as absolute file URLs like
// 'C:\Path\file.Yaml'.
//
// Both are canonicalized with a "file://" scheme, slashes and a lower-cased path:
// 'file:///c:/path/file.yaml'
//
// URLs can be specified with a file scheme, like in 'file:///folder/file.json' or
// 'file:///c:\folder\File.json'.
//
// URLs like file://C:\folder are considered invalid (i.e. there is no host 'c:\folder') and a "repair"
// is attempted.
//
// The base path argument is assumed to be canonicalized (e.g. using normalizeBase()).
func normalizeURI(refPath, base string) string {
	refURL, err := parseURL(refPath)
	if err != nil {
		specLogger.Printf("warning: invalid URI in $ref  %q: %v", refPath, err)
		refURL, refPath = repairURI(refPath)
	}

	fixWindowsURI(refURL, refPath) // noop on non-windows OS

	refURL.Path = path.Clean(refURL.Path)
	if refURL.Path == "." {
		refURL.Path = ""
	}
	forgetSpelling(refURL)

	r := MustCreateRef(refURL.String())
	if r.IsCanonical() {
		return r.String()
	}

	baseURL, _ := parseURL(base)
	if path.IsAbs(refURL.Path) {
		baseURL.Path = refURL.Path
	} else if refURL.Path != "" {
		baseURL.Path = path.Join(path.Dir(baseURL.Path), refURL.Path)
	}
	// copying fragment from ref to base
	baseURL.Fragment = refURL.Fragment

	if baseURL.Scheme != "" && baseURL.Path != "" && !path.IsAbs(baseURL.Path) {
		// a base that carries no path of its own (e.g. "smb://host") leaves the join relative.
		// Anchor it, because a relative path under a scheme does not survive rendering: it is
		// either promoted to a host component, which is not even guaranteed to be a legal one
		// ("a://some file.json" no longer parses), or read back as absolute anyway.
		baseURL.Path = path.Join("/", baseURL.Path)
	}

	return baseURL.String()
}

// denormalizeRef returns the simplest notation for a normalized $ref, given the path of the original root document.
//
// When calling this, we assume that:
// * $ref is a canonical URI
// * originalRelativeBase is a canonical URI
//
// denormalizeRef is currently used when we rewrite a $ref after a circular $ref has been detected.
// In this case, expansion stops and normally renders the internal canonical $ref.
//
// This internal $ref is eventually rebased to the original RelativeBase used for the expansion.
//
// There is a special case for schemas that are anchored with an "id":
// in that case, the rebasing is performed // against the id only if this is an anchor for the initial root document.
// All other intermediate "id"'s found along the way are ignored for the purpose of rebasing.
func denormalizeRef(ref *Ref, originalRelativeBase, id string) Ref {
	debugLog("denormalizeRef called:\n$ref: %q\noriginal: %s\nroot ID:%s", ref.String(), originalRelativeBase, id)

	if ref.String() == "" || ref.IsRoot() || ref.HasFragmentOnly {
		// short circuit: $ref to current doc
		return *ref
	}

	if id != "" {
		if _, err := parseURL(id); err == nil { // if the schema id is not usable as a URI, ignore it
			// rebase, but keep references to root unchanged (do not want $ref: "")
			if ref, ok := rebase(ref, canonicalURL(id), true); ok {
				// $ref relative to the ID of the schema in the root document
				return ref
			}
		}
	}

	r, _ := rebase(ref, canonicalURL(originalRelativeBase), false)

	return r
}

// canonicalURL parses a URI the way a Ref does, so that rebase compares authorities that are
// spelled alike.
//
// Turning a URI into a Ref lower-cases the host and drops a default port, whereas the normalizer
// keeps the spelling it was handed: comparing the two spellings directly makes a $ref look like
// it belongs to another host, and leaves it absolute when it should have been rebased.
func canonicalURL(in string) *url.URL {
	if ref, err := NewRef(in); err == nil {
		return ref.GetURL()
	}

	if u, err := parseURL(in); err == nil {
		return u
	}

	return &url.URL{}
}

// rebase expresses a $ref relative to v, which is either the URI of the base document or
// the "id" that anchors the root schema.
//
// The two differ in what v stands for. An "id" anchors a namespace, so a $ref below it is
// expressed relative to the id itself. A base is a document, so a $ref is expressed relative
// to the folder that holds it, and only a $ref to that very document collapses to an empty $ref.
func rebase(ref *Ref, v *url.URL, isID bool) (Ref, bool) {
	var newBase url.URL

	u := ref.GetURL()

	if u.Scheme != v.Scheme || u.Host != v.Host {
		return *ref, false
	}

	docPath := v.Path
	v.Path = path.Dir(v.Path)

	if v.Path == "." {
		v.Path = ""
	} else if !strings.HasSuffix(v.Path, "/") {
		v.Path += "/"
	}

	newBase.Fragment = u.Fragment

	switch {
	case isID:
		if after, ok := cutPathPrefix(u.Path, docPath); ok {
			newBase.Path = after
		} else {
			newBase.Path = strings.TrimPrefix(u.Path, v.Path)
		}

		if newBase.Path == "" && newBase.Fragment == "" {
			// do not want rebasing to end up in an empty $ref
			return *ref, false
		}

	case u.Path == docPath:
		// the $ref points to the base document itself
		newBase.Path = ""

	default:
		newBase.Path = strings.TrimPrefix(u.Path, v.Path)

		if newBase.Path == "" {
			// the $ref points to the folder that holds the base, not to a document:
			// a $ref with no path of its own would denote the base document instead
			return *ref, false
		}
	}

	if path.IsAbs(newBase.Path) {
		// whenever we end up with an absolute path, we render a whole URI
		newBase.Scheme = v.Scheme
		newBase.Host = v.Host
		newBase.User = u.User
		newBase.RawQuery = u.RawQuery
		newBase.ForceQuery = u.ForceQuery
	} else if u.RawQuery != v.RawQuery || u.ForceQuery != v.ForceQuery || !sameUserinfo(u.User, v.User) {
		// a relative $ref carries neither credentials nor query of its own: normalizing one
		// applies those of the base. Leave the $ref absolute rather than have it resolve
		// against a different query or different credentials.
		return *ref, false
	}

	return MustCreateRef(newBase.String()), true
}

// sameUserinfo compares the userinfo components of two URIs.
//
// An absent userinfo differs from an empty one: "file:///x" and "file://@/x" do not render alike.
func sameUserinfo(a, b *url.Userinfo) bool {
	if a == nil || b == nil {
		return a == b
	}

	return a.String() == b.String()
}

// cutPathPrefix is strings.CutPrefix, cutting on path separators only.
//
// "/base/spec.json" is a prefix of "/base/spec.json/x" but not of "/base/spec.json.orig":
// cutting the latter would leave ".orig" to be rebased against the wrong folder.
func cutPathPrefix(pth, prefix string) (string, bool) {
	after, ok := strings.CutPrefix(pth, prefix)
	if !ok {
		return "", false
	}

	if after == "" || strings.HasPrefix(after, "/") || strings.HasSuffix(prefix, "/") {
		return after, true
	}

	return "", false
}

// normalizeRef canonicalize a Ref, using a canonical relativeBase as its absolute anchor.
func normalizeRef(ref *Ref, relativeBase string) *Ref {
	r := MustCreateRef(normalizeURI(ref.String(), relativeBase))
	return &r
}

// normalizeBase performs a normalization of the input base path.
//
// This always yields a canonical URI (absolute), usable for the document cache.
//
// It ensures that all further internal work on basePath may safely assume
// a non-empty, cross-platform, canonical URI (i.e. absolute).
//
// This normalization tolerates windows paths (e.g. C:\x\y\File.dat) and transform this
// in a file:// URL with lower cased drive letter and path.
//
// See also: https://en.wikipedia.org/wiki/File_URI_scheme
func normalizeBase(in string) string {
	u, err := parseURL(in)
	if err != nil {
		specLogger.Printf("warning: invalid URI in RelativeBase  %q: %v", in, err)
		u, in = repairURI(in)
	}

	u.Fragment = "" // any fragment in the base is irrelevant

	fixWindowsURI(u, in) // noop on non-windows OS

	u.Path = path.Clean(u.Path)
	if u.Path == "." { // empty after Clean()
		u.Path = ""
	}
	forgetSpelling(u)

	if u.Scheme != "" {
		if path.IsAbs(u.Path) || u.Scheme != fileScheme {
			// this is absolute or explicitly not a local file: we're good
			return canonicalString(u)
		}
	}

	// no scheme or file scheme with relative path: assume file and make it absolute
	// enforce scheme file://... with absolute path.
	//
	// If the input path is relative, we anchor the path to the current working directory.
	// NOTE: we may end up with a host component. Leave it unchanged: e.g. file://host/folder/file.json

	u.Scheme = fileScheme
	u.Path = absPath(u.Path) // platform-dependent
	u.RawQuery = ""          // any query component is irrelevant for a base
	return canonicalString(u)
}

// forgetSpelling drops the escaped spelling url.Parse recorded, so that rendering the URL
// derives it from Path and Fragment again.
//
// url.URL.String returns RawPath whenever it still decodes to Path, and the normalizer rewrites
// Path - path.Clean, then a join onto the base. A RawPath describing the path as it was written
// no longer describes the path we now hold, and rendering it can produce a URI that does not
// parse: normalizeBase("%2F") used to return "file://%2F", where the escaped form lost the
// leading slash that Path carries and what is left reads as an authority.
func forgetSpelling(u *url.URL) {
	u.RawPath = ""
	u.RawFragment = ""
}

// canonicalString renders a URL the way jsonreference does.
//
// The normalizer and jsonreference each canonicalize a URI, and they used to disagree: the
// normalizer kept the spelling it was handed while a Ref lower-cases the host, drops a default
// port and re-escapes path and fragment. Everything the expander stores - cache keys, the
// parentRefs chain, a $ref written back into the document - goes through both, so the two must
// agree or one document splits into two spellings of itself.
//
// A URI that jsonreference cannot parse is returned as it stands, rather than panicking through
// MustCreateRef. Such a URI is a defect in the normalizer, and the fuzz target reports it as one.
func canonicalString(u *url.URL) string {
	rendered := u.String()

	r, err := NewRef(rendered)
	if err != nil {
		specLogger.Printf("warning: the normalizer produced an unparseable URI %q: %v", rendered, err)

		return rendered
	}

	return r.String()
}
