package registrar

import (
	"reflect"
	"testing"
)

func TestReserve(t *testing.T) {
	r := NewRegistrar()

	obj := "test1"
	if err := r.Reserve("test", obj); err != nil {
		t.Fatal(err)
	}

	if err := r.Reserve("test", obj); err != nil {
		t.Fatal(err)
	}

	obj2 := "test2"
	err := r.Reserve("test", obj2)
	if err == nil {
		t.Fatalf("expected error when reserving an already reserved name to another object")
	}
	if err != ErrNameReserved {
		t.Fatal("expected `ErrNameReserved` error when attempting to reserve an already reserved name")
	}
}

func TestRelease(t *testing.T) {
	r := NewRegistrar()
	obj := "testing"

	if err := r.Reserve("test", obj); err != nil {
		t.Fatal(err)
	}
	r.Release("test")
	r.Release("test") // Ensure there is no panic here

	if err := r.Reserve("test", obj); err != nil {
		t.Fatal(err)
	}
}

func TestGetNames(t *testing.T) {
	r := NewRegistrar()
	obj := "testing"
	names := []string{"test1", "test2"}

	for _, name := range names {
		if err := r.Reserve(name, obj); err != nil {
			t.Fatal(err)
		}
	}
	r.Reserve("test3", "other")

	names2, err := r.GetNames(obj)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(names, names2) {
		t.Fatalf("Exepected: %v, Got: %v", names, names2)
	}
}

func TestDelete(t *testing.T) {
	r := NewRegistrar()
	obj := "testing"
	names := []string{"test1", "test2"}
	for _, name := range names {
		if err := r.Reserve(name, obj); err != nil {
			t.Fatal(err)
		}
	}

	r.Reserve("test3", "other")
	r.Delete(obj)

	_, err := r.GetNames(obj)
	if err == nil {
		t.Fatal("expected error getting names for deleted key")
	}

	if err != ErrNoSuchKey {
		t.Fatal("expected `ErrNoSuchKey`")
	}
}

func TestGet(t *testing.T) {
	r := NewRegistrar()
	obj := "testing"
	name := "test"

	_, err := r.Get(name)
	if err == nil {
		t.Fatal("expected error when key does not exist")
	}
	if err != ErrNameNotReserved {
		t.Fatal(err)
	}

	if err := r.Reserve(name, obj); err != nil {
		t.Fatal(err)
	}

	if _, err = r.Get(name); err != nil {
		t.Fatal(err)
	}

	r.Delete(obj)
	_, err = r.Get(name)
	if err == nil {
		t.Fatal("expected error when key does not exist")
	}
	if err != ErrNameNotReserved {
		t.Fatal(err)
	}
}
