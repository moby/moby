// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !cgo
// +build !cgo

package x509

func loadSystemRoots() (*CertPool, error) {
	return execSecurityRoots()
}
