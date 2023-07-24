// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package common

type SnippetRangePointer struct {
	// 5.3: Snippet Byte Range: [start byte]:[end byte]
	// Cardinality: mandatory, one
	Offset int `json:"offset,omitempty"`

	// 5.4: Snippet Line Range: [start line]:[end line]
	// Cardinality: optional, one
	LineNumber int `json:"lineNumber,omitempty"`

	FileSPDXIdentifier ElementID `json:"reference"`
}

type SnippetRange struct {
	StartPointer SnippetRangePointer `json:"startPointer"`
	EndPointer   SnippetRangePointer `json:"endPointer"`
}
