package idm

import (
	"testing"

	_ "github.com/docker/libnetwork/testutils"
)

func TestNew(t *testing.T) {
	_, err := New(nil, "", 0, 1)
	if err == nil {
		t.Fatal("Expected failure, but succeeded")
	}

	_, err = New(nil, "myset", 1<<10, 0)
	if err == nil {
		t.Fatal("Expected failure, but succeeded")
	}

	i, err := New(nil, "myset", 0, 10)
	if err != nil {
		t.Fatalf("Unexpected failure: %v", err)
	}
	if i.handle == nil {
		t.Fatal("set is not initialized")
	}
	if i.start != 0 {
		t.Fatal("unexpected start")
	}
	if i.end != 10 {
		t.Fatal("unexpected end")
	}
}

func TestAllocate(t *testing.T) {
	i, err := New(nil, "myids", 50, 52)
	if err != nil {
		t.Fatal(err)
	}

	if err = i.GetSpecificID(49); err == nil {
		t.Fatal("Expected failure but succeeded")
	}

	if err = i.GetSpecificID(53); err == nil {
		t.Fatal("Expected failure but succeeded")
	}

	o, err := i.GetID()
	if err != nil {
		t.Fatal(err)
	}
	if o != 50 {
		t.Fatalf("Unexpected first id returned: %d", o)
	}

	err = i.GetSpecificID(50)
	if err == nil {
		t.Fatal(err)
	}

	o, err = i.GetID()
	if err != nil {
		t.Fatal(err)
	}
	if o != 51 {
		t.Fatalf("Unexpected id returned: %d", o)
	}

	o, err = i.GetID()
	if err != nil {
		t.Fatal(err)
	}
	if o != 52 {
		t.Fatalf("Unexpected id returned: %d", o)
	}

	o, err = i.GetID()
	if err == nil {
		t.Fatalf("Expected failure but succeeded: %d", o)
	}

	i.Release(50)

	o, err = i.GetID()
	if err != nil {
		t.Fatal(err)
	}
	if o != 50 {
		t.Fatal("Unexpected id returned")
	}

	i.Release(52)
	err = i.GetSpecificID(52)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUninitialized(t *testing.T) {
	i := &Idm{}

	if _, err := i.GetID(); err == nil {
		t.Fatal("Expected failure but succeeded")
	}

	if err := i.GetSpecificID(44); err == nil {
		t.Fatal("Expected failure but succeeded")
	}
}
