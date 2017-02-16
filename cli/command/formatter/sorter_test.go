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

	whitelist := map[string]string{"S": ""}
	gs, err := newGenericStructSorter(data, []string{"S"}, whitelist)
	if err != nil {
		t.Fatal("failed to create sorter: ", err)
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

	whitelist := map[string]string{"B": ""}
	gs, err := newGenericStructSorter(data, []string{"B:desc"}, whitelist)
	if err != nil {
		t.Fatal("failed to create sorter: ", err)
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

	whitelist := map[string]string{"I": ""}
	gs, err := newGenericStructSorter(data, []string{"I:asc"}, whitelist)
	if err != nil {
		t.Fatal("failed to create sorter: ", err)
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

	whitelist := map[string]string{"U": ""}
	gs, err := newGenericStructSorter(data, []string{"U"}, whitelist)
	if err != nil {
		t.Fatal("failed to create sorter: ", err)
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

	whitelist := map[string]string{"F": ""}
	gs, err := newGenericStructSorter(data, []string{"F"}, whitelist)
	if err != nil {
		t.Fatal("failed to create sorter: ", err)
	}

	sort.Sort(gs)

	if !reflect.DeepEqual(gs.data, expected) {
		t.Fatalf("sorting failed:\nexpected: %#v\ngot: %#v", expected, gs.data)
	}
}

func TestTimeStructSorter(t *testing.T) {
	type timeStruct struct {
		name string
		T    time.Time
	}

	sc, _ := time.LoadLocation("America/Los_Angeles")

	ascExpected := []interface{}{
		timeStruct{name: "a", T: time.Date(1984, time.September, 27, 13, 0, 0, 0, time.UTC)},
		timeStruct{name: "b", T: time.Date(1984, time.September, 27, 13, 0, 0, 0, time.UTC)},
		timeStruct{name: "c", T: time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)},
		timeStruct{name: "d", T: time.Date(2013, time.March, 13, 13, 20, 0, 0, sc)},
		timeStruct{name: "e", T: time.Date(2015, time.December, 15, 9, 0, 0, 0, sc)},
	}
	descExpected := []interface{}{
		timeStruct{name: "e", T: time.Date(2015, time.December, 15, 9, 0, 0, 0, sc)},
		timeStruct{name: "d", T: time.Date(2013, time.March, 13, 13, 20, 0, 0, sc)},
		timeStruct{name: "c", T: time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)},
		timeStruct{name: "a", T: time.Date(1984, time.September, 27, 13, 0, 0, 0, time.UTC)},
		timeStruct{name: "b", T: time.Date(1984, time.September, 27, 13, 0, 0, 0, time.UTC)},
	}
	data := []interface{}{
		timeStruct{name: "d", T: time.Date(2013, time.March, 13, 13, 20, 0, 0, sc)},
		timeStruct{name: "a", T: time.Date(1984, time.September, 27, 13, 0, 0, 0, time.UTC)},
		timeStruct{name: "b", T: time.Date(1984, time.September, 27, 13, 0, 0, 0, time.UTC)},
		timeStruct{name: "e", T: time.Date(2015, time.December, 15, 9, 0, 0, 0, sc)},
		timeStruct{name: "c", T: time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)},
	}

	whitelist := map[string]string{"T": ""}
	gs, err := newGenericStructSorter(data, []string{"T"}, whitelist)
	if err != nil {
		t.Fatal("failed to create sorter:", err)
	}

	sort.Sort(gs)

	if !reflect.DeepEqual(gs.data, ascExpected) {
		t.Fatalf("sorting failed:\nexpected: %#v\ngot: %#v", ascExpected, gs.data)
	}

	gs, err = newGenericStructSorter(data, []string{"T:desc"}, whitelist)
	if err != nil {
		t.Fatal("failed to create sorter:", err)
	}

	sort.Sort(gs)

	if !reflect.DeepEqual(gs.data, descExpected) {
		t.Fatalf("sorting failed:\nexpected: %#v\ngot: %#v", descExpected, gs.data)
	}
}

func TestPtrStructSorter(t *testing.T) {
	type innerStruct struct {
		innerValue string
	}
	type ptrStruct struct {
		name  string
		value *innerStruct
	}

	expected := []interface{}{
		ptrStruct{"a", nil}, ptrStruct{"b", nil}, ptrStruct{"c", &innerStruct{"innerA"}}, ptrStruct{"d", &innerStruct{"innerB"}},
	}
	data := []interface{}{
		ptrStruct{"a", nil}, ptrStruct{"d", &innerStruct{"innerB"}}, ptrStruct{"c", &innerStruct{"innerA"}}, ptrStruct{"b", nil},
	}

	whitelist := map[string]string{"innerValue": ""}
	gs, err := newGenericStructSorter(data, []string{"innerValue"}, whitelist)
	if err != nil {
		t.Fatal("failed to create sorter:", err)
	}

	sort.Sort(gs)

	if !reflect.DeepEqual(gs.data, expected) {
		t.Fatalf("sorting failed:\nexpected: %#v\ngot: %#v", expected, gs.data)
	}
}

func TestGetSortableFields(t *testing.T) {
	type inInnerStruct struct {
		Name           string
		deepInnerValue int
	}

	type innerStruct struct {
		innerValue int
		inInnerS   inInnerStruct
	}
	type complexStruct struct {
		name   string
		Name   *string
		innerS *innerStruct
		t      time.Time
	}

	var cs complexStruct
	whitelist := map[string]string{
		"Name":           "MyName",
		"t":              "Time",
		"deepInnerValue": "DeepValue",
	}
	sortableFields := map[string]fieldIndex{}
	if err := getSortableFields(reflect.TypeOf(cs), whitelist, nil, sortableFields); err != nil {
		t.Fatalf("failed to get sortable fields: %v", err)
	}

	expected := map[string]fieldIndex{
		"myname": {1}, "time": {3}, "deepvalue": {2, 1, 1},
	}

	if !reflect.DeepEqual(sortableFields, expected) {
		t.Fatalf("get sortable fields failed:\nexpected: %#v\ngot: %#v", expected, sortableFields)
	}
}
