// Copyright 2017, The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build !go1.8

package cmpopts

import (
	"reflect"
	"sort"
)

const hasReflectStructOf = false

func mapEntryType(reflect.Type) reflect.Type {
	return reflect.TypeOf(struct{ K, V interface{} }{})
}

func sliceIsSorted(slice interface{}, less func(i, j int) bool) bool {
	return sort.IsSorted(reflectSliceSorter{reflect.ValueOf(slice), less})
}
func sortSlice(slice interface{}, less func(i, j int) bool) {
	sort.Sort(reflectSliceSorter{reflect.ValueOf(slice), less})
}
func sortSliceStable(slice interface{}, less func(i, j int) bool) {
	sort.Stable(reflectSliceSorter{reflect.ValueOf(slice), less})
}

type reflectSliceSorter struct {
	slice reflect.Value
	less  func(i, j int) bool
}

func (ss reflectSliceSorter) Len() int {
	return ss.slice.Len()
}
func (ss reflectSliceSorter) Less(i, j int) bool {
	return ss.less(i, j)
}
func (ss reflectSliceSorter) Swap(i, j int) {
	vi := ss.slice.Index(i).Interface()
	vj := ss.slice.Index(j).Interface()
	ss.slice.Index(i).Set(reflect.ValueOf(vj))
	ss.slice.Index(j).Set(reflect.ValueOf(vi))
}
