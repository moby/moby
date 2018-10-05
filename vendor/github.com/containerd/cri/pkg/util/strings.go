/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import "strings"

// InStringSlice checks whether a string is inside a string slice.
// Comparison is case insensitive.
func InStringSlice(ss []string, str string) bool {
	for _, s := range ss {
		if strings.ToLower(s) == strings.ToLower(str) {
			return true
		}
	}
	return false
}

// SubtractStringSlice subtracts string from string slice.
// Comparison is case insensitive.
func SubtractStringSlice(ss []string, str string) []string {
	var res []string
	for _, s := range ss {
		if strings.ToLower(s) == strings.ToLower(str) {
			continue
		}
		res = append(res, s)
	}
	return res
}

// MergeStringSlices merges 2 string slices into one and remove duplicated elements.
func MergeStringSlices(a []string, b []string) []string {
	set := map[string]struct{}{}
	for _, s := range a {
		set[s] = struct{}{}
	}
	for _, s := range b {
		set[s] = struct{}{}
	}
	var ss []string
	for s := range set {
		ss = append(ss, s)
	}
	return ss
}
