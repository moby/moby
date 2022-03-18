//go:build gofuzz
// +build gofuzz

// Copyright 2019+ Klaus Post. All rights reserved.
// License information can be found in the LICENSE file.
// Based on work by Yann Collet, released under BSD License.

package zstd

// ignoreCRC can be used for fuzz testing to ignore CRC values...
const ignoreCRC = true
