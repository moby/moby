// Copyright (c) 2014 The go-patricia AUTHORS
//
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package patricia

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const (
	success = true
	failure = false
)

type testData struct {
	key    string
	value  interface{}
	retVal bool
}

// Tests -----------------------------------------------------------------------

func TestTrie_InsertDifferentPrefixes(t *testing.T) {
	trie := NewTrie()

	data := []testData{
		{"Pepaneeeeeeeeeeeeee", "Pepan Zdepan", success},
		{"Honzooooooooooooooo", "Honza Novak", success},
		{"Jenikuuuuuuuuuuuuuu", "Jenik Poustevnicek", success},
	}

	for _, v := range data {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert(Prefix(v.key), v.value); ok != v.retVal {
			t.Errorf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}
}

func TestTrie_InsertDuplicatePrefixes(t *testing.T) {
	trie := NewTrie()

	data := []testData{
		{"Pepan", "Pepan Zdepan", success},
		{"Pepan", "Pepan Zdepan", failure},
	}

	for _, v := range data {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert(Prefix(v.key), v.value); ok != v.retVal {
			t.Errorf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}
}

func TestTrie_InsertVariousPrefixes(t *testing.T) {
	trie := NewTrie()

	data := []testData{
		{"Pepan", "Pepan Zdepan", success},
		{"Pepin", "Pepin Omacka", success},
		{"Honza", "Honza Novak", success},
		{"Jenik", "Jenik Poustevnicek", success},
		{"Pepan", "Pepan Dupan", failure},
		{"Karel", "Karel Pekar", success},
		{"Jenik", "Jenik Poustevnicek", failure},
		{"Pepanek", "Pepanek Zemlicka", success},
	}

	for _, v := range data {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert(Prefix(v.key), v.value); ok != v.retVal {
			t.Errorf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}
}

func TestTrie_InsertAndMatchPrefix(t *testing.T) {
	trie := NewTrie()
	t.Log("INSERT prefix=by week")
	trie.Insert(Prefix("by week"), 2)
	t.Log("INSERT prefix=by")
	trie.Insert(Prefix("by"), 1)

	if !trie.Match(Prefix("by")) {
		t.Error("MATCH prefix=by, expected=true, got=false")
	}
}

func TestTrie_SetGet(t *testing.T) {
	trie := NewTrie()

	data := []testData{
		{"Pepan", "Pepan Zdepan", success},
		{"Pepin", "Pepin Omacka", success},
		{"Honza", "Honza Novak", success},
		{"Jenik", "Jenik Poustevnicek", success},
		{"Pepan", "Pepan Dupan", failure},
		{"Karel", "Karel Pekar", success},
		{"Jenik", "Jenik Poustevnicek", failure},
		{"Pepanek", "Pepanek Zemlicka", success},
	}

	for _, v := range data {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert(Prefix(v.key), v.value); ok != v.retVal {
			t.Errorf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}

	for _, v := range data {
		t.Logf("SET %q to 10", v.key)
		trie.Set(Prefix(v.key), 10)
	}

	for _, v := range data {
		value := trie.Get(Prefix(v.key))
		t.Logf("GET %q => %v", v.key, value)
		if value.(int) != 10 {
			t.Errorf("Unexpected return value, != 10", value)
		}
	}

	if value := trie.Get(Prefix("random crap")); value != nil {
		t.Errorf("Unexpected return value, %v != <nil>", value)
	}
}

func TestTrie_Match(t *testing.T) {
	trie := NewTrie()

	data := []testData{
		{"Pepan", "Pepan Zdepan", success},
		{"Pepin", "Pepin Omacka", success},
		{"Honza", "Honza Novak", success},
		{"Jenik", "Jenik Poustevnicek", success},
		{"Pepan", "Pepan Dupan", failure},
		{"Karel", "Karel Pekar", success},
		{"Jenik", "Jenik Poustevnicek", failure},
		{"Pepanek", "Pepanek Zemlicka", success},
	}

	for _, v := range data {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert(Prefix(v.key), v.value); ok != v.retVal {
			t.Errorf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}

	for _, v := range data {
		matched := trie.Match(Prefix(v.key))
		t.Logf("MATCH %q => %v", v.key, matched)
		if !matched {
			t.Errorf("Inserted key %q was not matched", v.key)
		}
	}

	if trie.Match(Prefix("random crap")) {
		t.Errorf("Key that was not inserted matched: %q", "random crap")
	}
}

func TestTrie_MatchFalsePositive(t *testing.T) {
	trie := NewTrie()

	if ok := trie.Insert(Prefix("A"), 1); !ok {
		t.Fatal("INSERT prefix=A, item=1 not ok")
	}

	resultMatchSubtree := trie.MatchSubtree(Prefix("A extra"))
	resultMatch := trie.Match(Prefix("A extra"))

	if resultMatchSubtree != false {
		t.Error("MatchSubtree returned false positive")
	}

	if resultMatch != false {
		t.Error("Match returned false positive")
	}
}

func TestTrie_MatchSubtree(t *testing.T) {
	trie := NewTrie()

	data := []testData{
		{"Pepan", "Pepan Zdepan", success},
		{"Pepin", "Pepin Omacka", success},
		{"Honza", "Honza Novak", success},
		{"Jenik", "Jenik Poustevnicek", success},
		{"Pepan", "Pepan Dupan", failure},
		{"Karel", "Karel Pekar", success},
		{"Jenik", "Jenik Poustevnicek", failure},
		{"Pepanek", "Pepanek Zemlicka", success},
	}

	for _, v := range data {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert(Prefix(v.key), v.value); ok != v.retVal {
			t.Errorf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}

	for _, v := range data {
		key := Prefix(v.key[:3])
		matched := trie.MatchSubtree(key)
		t.Logf("MATCH_SUBTREE %q => %v", key, matched)
		if !matched {
			t.Errorf("Subtree %q was not matched", v.key)
		}
	}
}

func TestTrie_Visit(t *testing.T) {
	trie := NewTrie()

	data := []testData{
		{"Pepa", 0, success},
		{"Pepa Zdepa", 1, success},
		{"Pepa Kuchar", 2, success},
		{"Honza", 3, success},
		{"Jenik", 4, success},
	}

	for _, v := range data {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert([]byte(v.key), v.value); ok != v.retVal {
			t.Fatalf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}

	if err := trie.Visit(func(prefix Prefix, item Item) error {
		name := data[item.(int)].key
		t.Logf("VISITING prefix=%q, item=%v", prefix, item)
		if !strings.HasPrefix(string(prefix), name) {
			t.Errorf("Unexpected prefix encountered, %q not a prefix of %q", prefix, name)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestTrie_VisitSkipSubtree(t *testing.T) {
	trie := NewTrie()

	data := []testData{
		{"Pepa", 0, success},
		{"Pepa Zdepa", 1, success},
		{"Pepa Kuchar", 2, success},
		{"Honza", 3, success},
		{"Jenik", 4, success},
	}

	for _, v := range data {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert([]byte(v.key), v.value); ok != v.retVal {
			t.Fatalf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}

	if err := trie.Visit(func(prefix Prefix, item Item) error {
		t.Logf("VISITING prefix=%q, item=%v", prefix, item)
		if item.(int) == 0 {
			t.Logf("SKIP %q", prefix)
			return SkipSubtree
		}
		if strings.HasPrefix(string(prefix), "Pepa") {
			t.Errorf("Unexpected prefix encountered, %q", prefix)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestTrie_VisitReturnError(t *testing.T) {
	trie := NewTrie()

	data := []testData{
		{"Pepa", 0, success},
		{"Pepa Zdepa", 1, success},
		{"Pepa Kuchar", 2, success},
		{"Honza", 3, success},
		{"Jenik", 4, success},
	}

	for _, v := range data {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert([]byte(v.key), v.value); ok != v.retVal {
			t.Fatalf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}

	someErr := errors.New("Something exploded")
	if err := trie.Visit(func(prefix Prefix, item Item) error {
		t.Logf("VISITING prefix=%q, item=%v", prefix, item)
		if item.(int) == 0 {
			return someErr
		}
		if item.(int) != 0 {
			t.Errorf("Unexpected prefix encountered, %q", prefix)
		}
		return nil
	}); err != nil && err != someErr {
		t.Fatal(err)
	}
}

func TestTrie_VisitSubtree(t *testing.T) {
	trie := NewTrie()

	data := []testData{
		{"Pepa", 0, success},
		{"Pepa Zdepa", 1, success},
		{"Pepa Kuchar", 2, success},
		{"Honza", 3, success},
		{"Jenik", 4, success},
	}

	for _, v := range data {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert([]byte(v.key), v.value); ok != v.retVal {
			t.Fatalf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}

	var counter int
	subtreePrefix := []byte("Pep")
	t.Log("VISIT Pep")
	if err := trie.VisitSubtree(subtreePrefix, func(prefix Prefix, item Item) error {
		t.Logf("VISITING prefix=%q, item=%v", prefix, item)
		if !bytes.HasPrefix(prefix, subtreePrefix) {
			t.Errorf("Unexpected prefix encountered, %q does not extend %q",
				prefix, subtreePrefix)
		}
		if len(prefix) > len(data[item.(int)].key) {
			t.Fatalf("Something is rather fishy here, prefix=%q", prefix)
		}
		counter++
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if counter != 3 {
		t.Error("Unexpected number of nodes visited")
	}
}

func TestTrie_VisitPrefixes(t *testing.T) {
	trie := NewTrie()

	data := []testData{
		{"P", 0, success},
		{"Pe", 1, success},
		{"Pep", 2, success},
		{"Pepa", 3, success},
		{"Pepa Zdepa", 4, success},
		{"Pepa Kuchar", 5, success},
		{"Honza", 6, success},
		{"Jenik", 7, success},
	}

	for _, v := range data {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert([]byte(v.key), v.value); ok != v.retVal {
			t.Fatalf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}

	var counter int
	word := []byte("Pepa")
	if err := trie.VisitPrefixes(word, func(prefix Prefix, item Item) error {
		t.Logf("VISITING prefix=%q, item=%v", prefix, item)
		if !bytes.HasPrefix(word, prefix) {
			t.Errorf("Unexpected prefix encountered, %q is not a prefix of %q",
				prefix, word)
		}
		counter++
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if counter != 4 {
		t.Error("Unexpected number of nodes visited")
	}
}

func TestParticiaTrie_Delete(t *testing.T) {
	trie := NewTrie()

	data := []testData{
		{"Pepan", "Pepan Zdepan", success},
		{"Honza", "Honza Novak", success},
		{"Jenik", "Jenik Poustevnicek", success},
	}

	for _, v := range data {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert([]byte(v.key), v.value); ok != v.retVal {
			t.Fatalf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}

	for _, v := range data {
		t.Logf("DELETE word=%v, success=%v", v.key, v.retVal)
		if ok := trie.Delete([]byte(v.key)); ok != v.retVal {
			t.Errorf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}
}

func TestParticiaTrie_DeleteNonExistent(t *testing.T) {
	trie := NewTrie()

	insertData := []testData{
		{"Pepan", "Pepan Zdepan", success},
		{"Honza", "Honza Novak", success},
		{"Jenik", "Jenik Poustevnicek", success},
	}
	deleteData := []testData{
		{"Pepan", "Pepan Zdepan", success},
		{"Honza", "Honza Novak", success},
		{"Pepan", "Pepan Zdepan", failure},
		{"Jenik", "Jenik Poustevnicek", success},
		{"Honza", "Honza Novak", failure},
	}

	for _, v := range insertData {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert([]byte(v.key), v.value); ok != v.retVal {
			t.Fatalf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}

	for _, v := range deleteData {
		t.Logf("DELETE word=%v, success=%v", v.key, v.retVal)
		if ok := trie.Delete([]byte(v.key)); ok != v.retVal {
			t.Errorf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}
}

func TestParticiaTrie_DeleteSubtree(t *testing.T) {
	trie := NewTrie()

	insertData := []testData{
		{"P", 0, success},
		{"Pe", 1, success},
		{"Pep", 2, success},
		{"Pepa", 3, success},
		{"Pepa Zdepa", 4, success},
		{"Pepa Kuchar", 5, success},
		{"Honza", 6, success},
		{"Jenik", 7, success},
	}
	deleteData := []testData{
		{"Pe", -1, success},
		{"Pe", -1, failure},
		{"Honzik", -1, failure},
		{"Honza", -1, success},
		{"Honza", -1, failure},
		{"Pep", -1, failure},
		{"P", -1, success},
		{"Nobody", -1, failure},
		{"", -1, success},
	}

	for _, v := range insertData {
		t.Logf("INSERT prefix=%v, item=%v, success=%v", v.key, v.value, v.retVal)
		if ok := trie.Insert([]byte(v.key), v.value); ok != v.retVal {
			t.Fatalf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}

	for _, v := range deleteData {
		t.Logf("DELETE_SUBTREE prefix=%v, success=%v", v.key, v.retVal)
		if ok := trie.DeleteSubtree([]byte(v.key)); ok != v.retVal {
			t.Errorf("Unexpected return value, expected=%v, got=%v", v.retVal, ok)
		}
	}
}

/*
func TestTrie_Dump(t *testing.T) {
	trie := NewTrie()

	data := []testData{
		{"Honda", nil, success},
		{"Honza", nil, success},
		{"Jenik", nil, success},
		{"Pepan", nil, success},
		{"Pepin", nil, success},
	}

	for i, v := range data {
		if _, ok := trie.Insert([]byte(v.key), v.value); ok != v.retVal {
			t.Logf("INSERT %v %v", v.key, v.value)
			t.Fatalf("Unexpected return value, expected=%v, got=%v", i, ok)
		}
	}

	dump := `
+--+--+ Hon +--+--+ da
   |           |
   |           +--+ za
   |
   +--+ Jenik
   |
   +--+ Pep +--+--+ an
               |
               +--+ in
`

	var buf bytes.Buffer
	trie.Dump(buf)

	if !bytes.Equal(buf.Bytes(), dump) {
		t.Logf("DUMP")
		t.Fatalf("Unexpected dump generated, expected\n\n%v\ngot\n\n%v", dump, buf.String())
	}
}
*/

func TestTrie_compact(t *testing.T) {
	trie := NewTrie()

	trie.Insert(Prefix("a"), 0)
	trie.Insert(Prefix("ab"), 0)
	trie.Insert(Prefix("abc"), 0)
	trie.Insert(Prefix("abcd"), 0)
	trie.Insert(Prefix("abcde"), 0)
	trie.Insert(Prefix("abcdef"), 0)
	trie.Insert(Prefix("abcdefg"), 0)
	trie.Insert(Prefix("abcdefgi"), 0)
	trie.Insert(Prefix("abcdefgij"), 0)
	trie.Insert(Prefix("abcdefgijk"), 0)

	trie.Delete(Prefix("abcdef"))
	trie.Delete(Prefix("abcde"))
	trie.Delete(Prefix("abcdefg"))

	trie.Delete(Prefix("a"))
	trie.Delete(Prefix("abc"))
	trie.Delete(Prefix("ab"))

	trie.Visit(func(prefix Prefix, item Item) error {
		// 97 ~~ 'a',
		for ch := byte(97); ch <= 107; ch++ {
			if c := bytes.Count(prefix, []byte{ch}); c > 1 {
				t.Errorf("%q appeared in %q %v times", ch, prefix, c)
			}
		}
		return nil
	})
}

func TestTrie_longestCommonPrefixLenght(t *testing.T) {
	trie := NewTrie()
	trie.prefix = []byte("1234567890")

	switch {
	case trie.longestCommonPrefixLength([]byte("")) != 0:
		t.Fail()
	case trie.longestCommonPrefixLength([]byte("12345")) != 5:
		t.Fail()
	case trie.longestCommonPrefixLength([]byte("123789")) != 3:
		t.Fail()
	case trie.longestCommonPrefixLength([]byte("12345678901")) != 10:
		t.Fail()
	}
}

// Examples --------------------------------------------------------------------

func ExampleTrie() {
	// Create a new tree.
	trie := NewTrie()

	// Insert some items.
	trie.Insert(Prefix("Pepa Novak"), 1)
	trie.Insert(Prefix("Pepa Sindelar"), 2)
	trie.Insert(Prefix("Karel Macha"), 3)
	trie.Insert(Prefix("Karel Hynek Macha"), 4)

	// Just check if some things are present in the tree.
	key := Prefix("Pepa Novak")
	fmt.Printf("%q present? %v\n", key, trie.Match(key))
	key = Prefix("Karel")
	fmt.Printf("Anybody called %q here? %v\n", key, trie.MatchSubtree(key))

	// Walk the tree.
	trie.Visit(printItem)
	// "Pepa Novak": 1
	// "Pepa Sindelar": 2
	// "Karel Macha": 3
	// "Karel Hynek Macha": 4

	// Walk a subtree.
	trie.VisitSubtree(Prefix("Pepa"), printItem)
	// "Pepa Novak": 1
	// "Pepa Sindelar": 2

	// Modify an item, then fetch it from the tree.
	trie.Set(Prefix("Karel Hynek Macha"), 10)
	key = Prefix("Karel Hynek Macha")
	fmt.Printf("%q: %v\n", key, trie.Get(key))
	// "Karel Hynek Macha": 10

	// Walk prefixes.
	prefix := Prefix("Karel Hynek Macha je kouzelnik")
	trie.VisitPrefixes(prefix, printItem)
	// "Karel Hynek Macha": 10

	// Delete some items.
	trie.Delete(Prefix("Pepa Novak"))
	trie.Delete(Prefix("Karel Macha"))

	// Walk again.
	trie.Visit(printItem)
	// "Pepa Sindelar": 2
	// "Karel Hynek Macha": 10

	// Delete a subtree.
	trie.DeleteSubtree(Prefix("Pepa"))

	// Print what is left.
	trie.Visit(printItem)
	// "Karel Hynek Macha": 10

	// Output:
	// "Pepa Novak" present? true
	// Anybody called "Karel" here? true
	// "Pepa Novak": 1
	// "Pepa Sindelar": 2
	// "Karel Macha": 3
	// "Karel Hynek Macha": 4
	// "Pepa Novak": 1
	// "Pepa Sindelar": 2
	// "Karel Hynek Macha": 10
	// "Karel Hynek Macha": 10
	// "Pepa Sindelar": 2
	// "Karel Hynek Macha": 10
	// "Karel Hynek Macha": 10
}

// Helpers ---------------------------------------------------------------------

func printItem(prefix Prefix, item Item) error {
	fmt.Printf("%q: %v\n", prefix, item)
	return nil
}
