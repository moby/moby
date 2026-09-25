// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build zos
// +build zos

package x509

// Possible certificate files; stop after finding one.
var certFiles = []string{
	"/etc/cacert.pem", // IBM zOS default
}
