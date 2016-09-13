package formatter

import (
	"reflect"
	"testing"
)

type dummy struct {
}

func (d *dummy) Func1() string {
	return "Func1"
}

func (d *dummy) func2() string {
	return "func2(should not be marshalled)"
}

func (d *dummy) Func3() (string, int) {
	return "Func3(should not be marshalled)", -42
}

func (d *dummy) Func4() int {
	return 4
}

type dummyType string

func (d *dummy) Func5() dummyType {
	return dummyType("Func5")
}

func (d *dummy) FullHeader() string {
	return "FullHeader(should not be marshalled)"
}

var dummyExpected = map[string]interface{}{
	"Func1": "Func1",
	"Func4": 4,
	"Func5": dummyType("Func5"),
}

func TestMarshalMap(t *testing.T) {
	d := dummy{}
	m, err := marshalMap(&d)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dummyExpected, m) {
		t.Fatalf("expected %+v, got %+v",
			dummyExpected, m)
	}
}

func TestMarshalMapBad(t *testing.T) {
	if _, err := marshalMap(nil); err == nil {
		t.Fatal("expected an error (argument is nil)")
	}
	if _, err := marshalMap(dummy{}); err == nil {
		t.Fatal("expected an error (argument is non-pointer)")
	}
	x := 42
	if _, err := marshalMap(&x); err == nil {
		t.Fatal("expected an error (argument is a pointer to non-struct)")
	}
}
