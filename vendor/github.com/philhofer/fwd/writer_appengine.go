//go:build appengine
// +build appengine

package fwd

func unsafestr(s string) []byte { return []byte(s) }
