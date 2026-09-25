// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build js && wasm
// +build js,wasm

package x509

// Possible certificate files; stop after finding one.
var certFiles = []string{}

func loadSystemRoots() (*CertPool, error) {
	return NewCertPool(), nil
}

func (c *Certificate) systemVerify(opts *VerifyOptions) (chains [][]*Certificate, err error) {
	return nil, nil
}
