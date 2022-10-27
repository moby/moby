// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build loong64
// +build loong64

package socket

const (
	sysRECVMMSG = 0xf3
	sysSENDMMSG = 0x10d
)
