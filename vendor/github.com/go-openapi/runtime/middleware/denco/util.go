// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package denco

// NextSeparator returns an index of next separator in path.
func NextSeparator(path string, start int) int {
	for start < len(path) {
		if c := path[start]; c == '/' || c == TerminationCharacter {
			break
		}
		start++
	}
	return start
}
