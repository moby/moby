// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package loads

import (
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"

	"github.com/go-openapi/swag/loading"
)

const (
	// numConfinementOptions is the count of loading options appended to enforce confinement
	// (WithRoot + WithHTTPClient), used to size the bundled option slice.
	numConfinementOptions = 2

	defaultTLSHandshakeTimeout = 10 * time.Second
)

// RestrictedHTTPClient returns an [http.Client] that refuses, at dial time, to connect to
// loopback, private, link-local (including cloud-metadata endpoints such as 169.254.169.254),
// or unspecified addresses. A blocked connection fails with an error wrapping
// [ErrForbiddenAddress].
//
// The check runs in the dialer Control hook, after DNS resolution and before connect, so it
// also covers HTTP redirects and DNS rebinding — which a URL-string allowlist cannot. The
// client does not honor proxy environment variables, so the guard always inspects the real
// destination rather than a proxy address.
//
// This is the network half of the restricted loaders ([JSONDocRestricted],
// [JSONSpecRestricted], [SpecRestricted]). It may also be used directly with
// [github.com/go-openapi/swag/loading.WithHTTPClient].
//
// The policy is opinionated and deliberately simple. For a different one (a custom allow/deny
// list, an explicit proxy, mutual TLS, ...), build your own client and pass it with
// [github.com/go-openapi/swag/loading.WithHTTPClient]. To keep the default address policy as a
// base, reuse [IsForbiddenAddress] in your own dialer Control hook — see the package examples
// for the pattern.
func RestrictedHTTPClient() *http.Client {
	control := func(_, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		addr, err := netip.ParseAddr(host)
		if err != nil {
			return err
		}
		if IsForbiddenAddress(addr) {
			return ErrForbiddenAddress
		}

		return nil
	}

	return &http.Client{
		Transport: &http.Transport{
			Proxy:               nil, // dial the real destination so the guard inspects it
			DialContext:         (&net.Dialer{Control: control}).DialContext,
			ForceAttemptHTTP2:   true,
			TLSHandshakeTimeout: defaultTLSHandshakeTimeout,
		},
	}
}

// IsForbiddenAddress reports whether addr is one that [RestrictedHTTPClient] refuses to dial:
// a loopback, private, link-local (including cloud-metadata endpoints such as 169.254.169.254),
// or unspecified address. IPv4-mapped IPv6 addresses are unmapped before the check.
//
// It is exported so callers can reuse or extend the default policy when building their own
// dialer Control hook, for example to also reject a CGNAT range or to carve out a single
// trusted internal host:
//
//	control := func(_, address string, _ syscall.RawConn) error {
//		host, _, err := net.SplitHostPort(address)
//		if err != nil {
//			return err
//		}
//		addr, err := netip.ParseAddr(host)
//		if err != nil {
//			return err
//		}
//		if loads.IsForbiddenAddress(addr) && host != allowedInternalHost {
//			return loads.ErrForbiddenAddress
//		}
//		return nil
//	}
func IsForbiddenAddress(addr netip.Addr) bool {
	a := addr.Unmap()

	return a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsUnspecified()
}

// restrictedLoadingOptions bundles caller-supplied options with the confinement options,
// appended last so that local rooting and the restricted client always take precedence
// (the loading options are last-wins).
func restrictedLoadingOptions(root string, extra []loading.Option) []loading.Option {
	out := make([]loading.Option, 0, len(extra)+numConfinementOptions)
	out = append(out, extra...)
	out = append(out, loading.WithRoot(root), loading.WithHTTPClient(RestrictedHTTPClient()))

	return out
}

// JSONDocRestricted returns a JSON [DocLoader] that confines local reads to root (via
// [github.com/go-openapi/swag/loading.WithRoot]) and restricts remote fetches with
// [RestrictedHTTPClient].
//
// The returned loader may be registered with [WithDocLoader] or [AddLoader]. The confinement
// always takes precedence over any option passed here or at call time, so a caller cannot
// loosen it through [WithLoadingOptions].
//
// Like [JSONDoc], it loads JSON only: it does not convert YAML. For specs whose references may
// point at YAML documents, prefer [SpecRestricted], which keeps the default JSON/YAML chain.
func JSONDocRestricted(root string, opts ...loading.Option) DocLoader {
	// one restricted client, reused for every path and $ref
	return restrictedDocLoader(JSONDoc, restrictedLoadingOptions(root, opts))
}

// restrictedDocLoader wraps a [DocLoader] so that the confinement options in base are always
// applied, appended after any call-time options so they take precedence (loading options are
// last-wins).
func restrictedDocLoader(fn DocLoader, base []loading.Option) DocLoader {
	return func(path string, callOpts ...loading.Option) (json.RawMessage, error) {
		if len(callOpts) == 0 {
			return fn(path, base...)
		}

		all := make([]loading.Option, 0, len(callOpts)+len(base))
		all = append(all, callOpts...)
		all = append(all, base...) // confinement (tail of base) still wins

		return fn(path, all...)
	}
}

// JSONSpecRestricted loads a JSON spec like [JSONSpec], but confines local reads to root and
// restricts remote fetches with [RestrictedHTTPClient].
//
// The confinement is attached to the document's loader, so it also applies to every "$ref"
// resolved by [Document.Expanded]. Extra [github.com/go-openapi/swag/loading] options (custom
// headers, basic auth, timeout, ...) may be supplied; the confinement always wins over them.
func JSONSpecRestricted(path, root string, opts ...loading.Option) (*Document, error) {
	return JSONSpec(path, WithLoadingOptions(restrictedLoadingOptions(root, opts)...))
}

// SpecRestricted loads a spec like [Spec] — with JSON/YAML auto-detection — but confines local
// reads to root and restricts remote fetches with [RestrictedHTTPClient].
//
// The confinement is attached to the document's loader, so it also applies to every "$ref"
// resolved by [Document.Expanded]. Extra [github.com/go-openapi/swag/loading] options (custom
// headers, basic auth, timeout, ...) may be supplied; the confinement always wins over them.
func SpecRestricted(path, root string, opts ...loading.Option) (*Document, error) {
	return Spec(path, WithLoadingOptions(restrictedLoadingOptions(root, opts)...))
}

// SetRestrictedLoaders hardens the package-level default in a single call: it installs a
// confined JSON/YAML loader chain — local reads rooted at root, remote fetches through
// [RestrictedHTTPClient] — as the global default and as
// [github.com/go-openapi/spec.PathLoader].
//
// After this call, every load that relies on the package default ([Spec], [JSONSpec], and any
// cross-package "$ref" resolution) is confined, with no unconfined fallback left behind. It is
// the global counterpart of [SpecRestricted]; a single restricted client is shared across the
// chain. Extra [github.com/go-openapi/swag/loading] options may be supplied; the confinement
// always wins over them.
//
// # Concurrency
//
// Like [SetLoaders], this mutates package-level and [github.com/go-openapi/spec] globals and is
// not safe to call concurrently. Configure it once at startup, before serving. To revert, call
// [SetLoaders] with no arguments.
func SetRestrictedLoaders(root string, opts ...loading.Option) {
	base := restrictedLoadingOptions(root, opts) // one restricted client shared by the whole chain

	SetLoaders(
		NewDocLoaderWithMatch(restrictedDocLoader(loading.YAMLDoc, base), loading.YAMLMatcher),
		NewDocLoaderWithMatch(restrictedDocLoader(JSONDoc, base), nil), // nil matcher: JSON catch-all fallback
	)
}
