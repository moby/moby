package types

import (
	"testing"

	_ "github.com/docker/libnetwork/netutils"
)

func TestErrorConstructors(t *testing.T) {
	var err error

	err = BadRequestErrorf("Io ho %d uccello", 1)
	if err.Error() != "Io ho 1 uccello" {
		t.Fatal(err)
	}
	if _, ok := err.(BadRequestError); !ok {
		t.Fatal(err)
	}
	if _, ok := err.(MaskableError); ok {
		t.Fatal(err)
	}

	err = NotFoundErrorf("Can't find the %s", "keys")
	if err.Error() != "Can't find the keys" {
		t.Fatal(err)
	}
	if _, ok := err.(NotFoundError); !ok {
		t.Fatal(err)
	}
	if _, ok := err.(MaskableError); ok {
		t.Fatal(err)
	}

	err = ForbiddenErrorf("Can't open door %d", 2)
	if err.Error() != "Can't open door 2" {
		t.Fatal(err)
	}
	if _, ok := err.(ForbiddenError); !ok {
		t.Fatal(err)
	}
	if _, ok := err.(MaskableError); ok {
		t.Fatal(err)
	}

	err = NotImplementedErrorf("Functionality %s is not implemented", "x")
	if err.Error() != "Functionality x is not implemented" {
		t.Fatal(err)
	}
	if _, ok := err.(NotImplementedError); !ok {
		t.Fatal(err)
	}
	if _, ok := err.(MaskableError); ok {
		t.Fatal(err)
	}

	err = TimeoutErrorf("Process %s timed out", "abc")
	if err.Error() != "Process abc timed out" {
		t.Fatal(err)
	}
	if _, ok := err.(TimeoutError); !ok {
		t.Fatal(err)
	}
	if _, ok := err.(MaskableError); ok {
		t.Fatal(err)
	}

	err = NoServiceErrorf("Driver %s is not available", "mh")
	if err.Error() != "Driver mh is not available" {
		t.Fatal(err)
	}
	if _, ok := err.(NoServiceError); !ok {
		t.Fatal(err)
	}
	if _, ok := err.(MaskableError); ok {
		t.Fatal(err)
	}

	err = InternalErrorf("Not sure what happened")
	if err.Error() != "Not sure what happened" {
		t.Fatal(err)
	}
	if _, ok := err.(InternalError); !ok {
		t.Fatal(err)
	}
	if _, ok := err.(MaskableError); ok {
		t.Fatal(err)
	}

	err = InternalMaskableErrorf("Minor issue, it can be ignored")
	if err.Error() != "Minor issue, it can be ignored" {
		t.Fatal(err)
	}
	if _, ok := err.(InternalError); !ok {
		t.Fatal(err)
	}
	if _, ok := err.(MaskableError); !ok {
		t.Fatal(err)
	}
}
