// Copyright (c) 2014 The go-patricia AUTHORS
//
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package patricia

import (
	"testing"
)

// Tests -----------------------------------------------------------------------

func TestTrie_InsertDense(t *testing.T) {
	trie := NewTrie()

	data := []testData{
		{"aba", 0, success},
		{"abb", 1, success},
		{"abc", 2, success},
		{"abd", 3, success},
		{"abe", 4, success},
		{"abf", 5, success},
		{"abg", 6, success},
		{"abh", 7, success},
		{"abi", 8, success},
		{"abj", 9, success},
		{"abk", 0, success},
		{"abl", 1, success},
		{"abm", 2, success},
		{"abn", 3, success},
		{"abo", 4, success},
		{"abp", 5, success},
		{"abq", 6, success},
		{"abr", 7, success},
		{"abs", 8, success},
		{"abt", 9, success},
		{"abu", 0, success},
		{"abv", 1, success},
		{"abw", 2, success},
		{"abx", 3, success},
		{"aby", 4, success},
		{"abz", 5, success},
	}

	for _, v := range data {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert(Prefix(v.key), v.value); ok != v.retVal {
			t.Errorf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}
}

func TestTrie_InsertDensePreceeding(t *testing.T) {
	trie := NewTrie()
	start := byte(70)
	// create a dense node
	for i := byte(0); i <= DefaultMaxChildrenPerSparseNode; i++ {
		if !trie.Insert(Prefix([]byte{start + i}), true) {
			t.Errorf("insert failed, prefix=%v", start+i)
		}
	}
	// insert some preceeding keys
	for i := byte(1); i < start; i *= i + 1 {
		if !trie.Insert(Prefix([]byte{start - i}), true) {
			t.Errorf("insert failed, prefix=%v", start-i)
		}
	}
}

func TestTrie_InsertDenseDuplicatePrefixes(t *testing.T) {
	trie := NewTrie()

	data := []testData{
		{"aba", 0, success},
		{"abb", 1, success},
		{"abc", 2, success},
		{"abd", 3, success},
		{"abe", 4, success},
		{"abf", 5, success},
		{"abg", 6, success},
		{"abh", 7, success},
		{"abi", 8, success},
		{"abj", 9, success},
		{"abk", 0, success},
		{"abl", 1, success},
		{"abm", 2, success},
		{"abn", 3, success},
		{"abo", 4, success},
		{"abp", 5, success},
		{"abq", 6, success},
		{"abr", 7, success},
		{"abs", 8, success},
		{"abt", 9, success},
		{"abu", 0, success},
		{"abv", 1, success},
		{"abw", 2, success},
		{"abx", 3, success},
		{"aby", 4, success},
		{"abz", 5, success},
		{"aba", 0, failure},
		{"abb", 1, failure},
		{"abc", 2, failure},
		{"abd", 3, failure},
		{"abe", 4, failure},
	}

	for _, v := range data {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert(Prefix(v.key), v.value); ok != v.retVal {
			t.Errorf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}
}

func TestTrie_DeleteDense(t *testing.T) {
	trie := NewTrie()

	data := []testData{
		{"aba", 0, success},
		{"abb", 1, success},
		{"abc", 2, success},
		{"abd", 3, success},
		{"abe", 4, success},
		{"abf", 5, success},
		{"abg", 6, success},
		{"abh", 7, success},
		{"abi", 8, success},
		{"abj", 9, success},
		{"abk", 0, success},
		{"abl", 1, success},
		{"abm", 2, success},
		{"abn", 3, success},
		{"abo", 4, success},
		{"abp", 5, success},
		{"abq", 6, success},
		{"abr", 7, success},
		{"abs", 8, success},
		{"abt", 9, success},
		{"abu", 0, success},
		{"abv", 1, success},
		{"abw", 2, success},
		{"abx", 3, success},
		{"aby", 4, success},
		{"abz", 5, success},
	}

	for _, v := range data {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert(Prefix(v.key), v.value); ok != v.retVal {
			t.Errorf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}

	for _, v := range data {
		t.Logf("DELETE word=%v, success=%v", v.key, v.retVal)
		if ok := trie.Delete([]byte(v.key)); ok != v.retVal {
			t.Errorf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}
}
