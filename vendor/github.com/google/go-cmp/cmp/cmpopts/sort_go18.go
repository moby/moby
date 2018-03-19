// Copyright 2017, The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build go1.8

package cmpopts

import (
	"reflect"
	"sort"
)

const hasReflectStructOf = true

func mapEntryType(t reflect.Type) reflect.Type {
	return reflect.StructOf([]reflect.StructField{
		{Name: "K", Type: t.Key()},
		{Name: "V", Type: t.Elem()},
	})
}

func sliceIsSorted(slice interface{}, less func(i, j int) bool) bool {
	return sort.SliceIsSorted(slice, less)
}
func sortSlice(slice interface{}, less func(i, j int) bool) {
	sort.Slice(slice, less)
}
func sortSliceStable(slice interface{}, less func(i, j int) bool) {
	sort.SliceStable(slice, less)
}
