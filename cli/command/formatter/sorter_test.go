package formatter

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestStringStructSorter(t *testing.T) {
	type strStruct struct {
		S string
	}

	expected := []interface{}{
		strStruct{"a"}, strStruct{"b"}, strStruct{"c"}, strStruct{"z"},
	}
	data := []interface{}{
		strStruct{"z"}, strStruct{"c"}, strStruct{"a"}, strStruct{"b"},
	}

	gs, err := newGenericStructSorter(data, []string{"S"})
	if err != nil {
		t.Fatal("failed to create sorter:", err)
	}

	sort.Sort(gs)

	if !reflect.DeepEqual(gs.data, expected) {
		t.Fatalf("sorting failed:\nexpected: %#v\ngot: %#v", expected, gs.data)
	}
}

func TestBoolStructSorter(t *testing.T) {
	type boolStruct struct {
		B bool
	}

	expected := []interface{}{
		boolStruct{true}, boolStruct{true}, boolStruct{false}, boolStruct{false},
	}
	data := []interface{}{
		boolStruct{true}, boolStruct{false}, boolStruct{true}, boolStruct{false},
	}

	gs, err := newGenericStructSorter(data, []string{"B:dsc"})
	if err != nil {
		t.Fatal("failed to create sorter:", err)
	}

	sort.Sort(gs)

	if !reflect.DeepEqual(gs.data, expected) {
		t.Fatalf("sorting failed:\nexpected: %#v\ngot: %#v", expected, gs.data)
	}
}

func TestIntStructSorter(t *testing.T) {
	type intStruct struct {
		I int
	}

	expected := []interface{}{
		intStruct{-21}, intStruct{10}, intStruct{16}, intStruct{42},
	}
	data := []interface{}{
		intStruct{42}, intStruct{16}, intStruct{-21}, intStruct{10},
	}

	gs, err := newGenericStructSorter(data, []string{"I:asc"})
	if err != nil {
		t.Fatal("failed to create sorter:", err)
	}

	sort.Sort(gs)

	if !reflect.DeepEqual(gs.data, expected) {
		t.Fatalf("sorting failed:\nexpected: %#v\ngot: %#v", expected, gs.data)
	}
}

func TestUintStructSorter(t *testing.T) {
	type uintStruct struct {
		U uint
	}

	expected := []interface{}{
		uintStruct{10}, uintStruct{16}, uintStruct{21}, uintStruct{42},
	}
	data := []interface{}{
		uintStruct{42}, uintStruct{16}, uintStruct{21}, uintStruct{10},
	}

	gs, err := newGenericStructSorter(data, []string{"U"})
	if err != nil {
		t.Fatal("failed to create sorter:", err)
	}

	sort.Sort(gs)

	if !reflect.DeepEqual(gs.data, expected) {
		t.Fatalf("sorting failed:\nexpected: %#v\ngot: %#v", expected, gs.data)
	}
}

func TestFloatStructSorter(t *testing.T) {
	type floatStruct struct {
		F float32
	}

	expected := []interface{}{
		floatStruct{-16.8}, floatStruct{3.14}, floatStruct{10}, floatStruct{21.42},
	}
	data := []interface{}{
		floatStruct{3.14}, floatStruct{-16.8}, floatStruct{21.42}, floatStruct{10},
	}

	gs, err := newGenericStructSorter(data, []string{"F"})
	if err != nil {
		t.Fatal("failed to create sorter:", err)
	}

	sort.Sort(gs)

	if !reflect.DeepEqual(gs.data, expected) {
		t.Fatalf("sorting failed:\nexpected: %#v\ngot: %#v", expected, gs.data)
	}
}

func TestTimeStructSorter(t *testing.T) {
	type timeStruct struct {
		T time.Time
	}

	sc, _ := time.LoadLocation("America/Los_Angeles")

	expected := []interface{}{
		timeStruct{time.Date(1984, time.September, 27, 13, 0, 0, 0, time.UTC)},
		timeStruct{time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)},
		timeStruct{time.Date(2013, time.March, 13, 13, 20, 0, 0, sc)},
		timeStruct{time.Date(2015, time.December, 15, 9, 0, 0, 0, sc)},
	}
	data := []interface{}{
		timeStruct{time.Date(2013, time.March, 13, 13, 20, 0, 0, sc)},
		timeStruct{time.Date(1984, time.September, 27, 13, 0, 0, 0, time.UTC)},
		timeStruct{time.Date(2015, time.December, 15, 9, 0, 0, 0, sc)},
		timeStruct{time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)},
	}

	gs, err := newGenericStructSorter(data, []string{"T"})
	if err != nil {
		t.Fatal("failed to create sorter:", err)
	}

	sort.Sort(gs)

	if !reflect.DeepEqual(gs.data, expected) {
		t.Fatalf("sorting failed:\nexpected: %#v\ngot: %#v", expected, gs.data)
	}
}
