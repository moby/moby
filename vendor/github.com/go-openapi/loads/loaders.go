// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package loads

import (
	"encoding/json"
	"errors"
	"net/url"
	"slices"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag/loading"
)

// Default chain of loaders, defined at the package level.
//
// By default this matches json and yaml documents.
//
// May be altered with AddLoader().
var loaders *loader

func init() {
	loaders = defaultLoaders()

	// sets the global default loader for go-openapi/spec
	spec.PathLoader = loaders.Load
}

// defaultLoaders builds the built-in loader chain: a YAML matcher first, with a JSON loader as
// the catch-all fallback.
func defaultLoaders() *loader {
	jsonLoader := &loader{
		DocLoaderWithMatch: DocLoaderWithMatch{
			Match: func(_ string) bool {
				return true
			},
			Fn: JSONDoc,
		},
	}

	return jsonLoader.WithHead(&loader{
		DocLoaderWithMatch: DocLoaderWithMatch{
			Match: loading.YAMLMatcher,
			Fn:    loading.YAMLDoc,
		},
	})
}

// buildLoaderChain links a list of [DocLoaderWithMatch] into a loader chain, preserving order.
// Entries with a nil Fn are skipped. Returns nil when no usable loader is provided.
func buildLoaderChain(ldrs ...DocLoaderWithMatch) *loader {
	var final, prev *loader
	for _, ldr := range ldrs {
		if ldr.Fn == nil {
			continue
		}

		node := &loader{DocLoaderWithMatch: ldr}
		if prev == nil {
			final = node
			prev = node

			continue
		}

		prev = prev.WithNext(node)
	}

	return final
}

// DocLoader represents a doc loader type.
type DocLoader func(string, ...loading.Option) (json.RawMessage, error)

// DocMatcher represents a predicate to check if a loader matches.
type DocMatcher func(string) bool

// DocLoaderWithMatch describes a loading function for a given extension match.
type DocLoaderWithMatch struct {
	Fn    DocLoader
	Match DocMatcher
}

// NewDocLoaderWithMatch builds a [DocLoaderWithMatch] to be used in load options.
func NewDocLoaderWithMatch(fn DocLoader, matcher DocMatcher) DocLoaderWithMatch {
	return DocLoaderWithMatch{
		Fn:    fn,
		Match: matcher,
	}
}

type loader struct {
	DocLoaderWithMatch

	loadingOptions []loading.Option

	Next *loader
}

// WithHead adds a loader at the head of the current stack.
func (l *loader) WithHead(head *loader) *loader {
	if head == nil {
		return l
	}
	head.Next = l
	return head
}

// WithNext adds a loader at the trail of the current stack.
func (l *loader) WithNext(next *loader) *loader {
	l.Next = next
	return next
}

// Load the raw document from path.
func (l *loader) Load(path string) (json.RawMessage, error) {
	_, erp := url.Parse(path)
	if erp != nil {
		return nil, errors.Join(erp, ErrLoads)
	}

	var lastErr error = ErrNoLoader // default error if no match was found
	for ldr := l; ldr != nil; ldr = ldr.Next {
		if ldr.Match != nil && !ldr.Match(path) {
			continue
		}

		// try then move to next one if there is an error
		b, err := ldr.Fn(path, l.loadingOptions...)
		if err == nil {
			return b, nil
		}

		lastErr = err
	}

	return nil, errors.Join(lastErr, ErrLoads)
}

func (l *loader) clone() *loader {
	if l == nil {
		return nil
	}

	return &loader{
		DocLoaderWithMatch: l.DocLoaderWithMatch,
		loadingOptions:     slices.Clone(l.loadingOptions),
		Next:               l.Next.clone(),
	}
}

// JSONDoc loads a json document from either a file or a remote URL.
//
// See [loading.Option] for available options (e.g. configuring authentication,
// headers or using embedded file system resources).
func JSONDoc(path string, opts ...loading.Option) (json.RawMessage, error) {
	data, err := loading.LoadFromFileOrHTTP(path, opts...)
	if err != nil {
		return nil, errors.Join(err, ErrLoads)
	}
	return json.RawMessage(data), nil
}

// AddLoader for a document, executed before other previously set loaders.
//
// This sets the configuration at the package level.
//
// # Concurrency
//
// This function updates the default loader used by [github.com/go-openapi/spec].
// Since this sets package level globals, you shouldn't call this concurrently.
//
// # Security
//
// AddLoader only *prepends* to the default chain: the previous loaders — including the
// unconfined JSON fallback — remain reachable, both here and via cross-package "$ref"
// resolution. It is therefore the wrong tool for hardening the global default. To replace the
// chain entirely (leaving no unconfined fallback) use [SetLoaders], or [SetRestrictedLoaders]
// for a one-call confined setup. For a single load, prefer a confined per-call loader via
// [WithLoadingOptions] or [WithDocLoaderMatches]. A custom loader registered here only honors
// the protections if its loading function applies the [github.com/go-openapi/swag/loading]
// options it is given. See the package documentation on Security.
func AddLoader(predicate DocMatcher, load DocLoader) {
	loaders = loaders.WithHead(&loader{
		DocLoaderWithMatch: DocLoaderWithMatch{
			Match: predicate,
			Fn:    load,
		},
	})

	// sets the global default loader for go-openapi/spec
	spec.PathLoader = loaders.Load
}

// SetLoaders replaces the package-level default loader chain with the given loaders, tried in
// order, and re-points [github.com/go-openapi/spec.PathLoader] at it.
//
// Unlike [AddLoader], nothing of the previous default survives — so when the replacement is
// confined, no unconfined fallback remains for any caller relying on the global default
// (including cross-package "$ref" resolution). An entry with a nil Match is a catch-all; you
// are responsible for providing a suitable fallback. Calling SetLoaders with no usable loader
// restores the built-in default (a YAML matcher with a JSON fallback).
//
// # Concurrency
//
// This sets package-level globals and the [github.com/go-openapi/spec] global loader. It is
// not safe to call concurrently with other loads or with [AddLoader]; configure it once at
// startup, before serving.
//
// # Security
//
// This is the way to harden the global default in one place. For a ready-made confined setup,
// see [SetRestrictedLoaders]. As with [AddLoader], a custom loader only honors the protections
// if its loading function applies the [github.com/go-openapi/swag/loading] options it is given.
// See the package documentation on Security.
func SetLoaders(ldrs ...DocLoaderWithMatch) {
	chain := buildLoaderChain(ldrs...)
	if chain == nil {
		chain = defaultLoaders()
	}

	loaders = chain

	// sets the global default loader for go-openapi/spec
	spec.PathLoader = loaders.Load
}
