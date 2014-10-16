package syncpipe

import (
	"fmt"
	"syscall"
	"testing"
)

type testStruct struct {
	Name string
}

func TestSendErrorFromChild(t *testing.T) {
	pipe, err := NewSyncPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := pipe.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	childfd, err := syscall.Dup(int(pipe.Child().Fd()))
	if err != nil {
		t.Fatal(err)
	}
	childPipe, _ := NewSyncPipeFromFd(0, uintptr(childfd))

	pipe.CloseChild()
	pipe.SendToChild(nil)

	expected := "something bad happened"
	childPipe.ReportChildError(fmt.Errorf(expected))

	childError := pipe.ReadFromChild()
	if childError == nil {
		t.Fatal("expected an error to be returned but did not receive anything")
	}

	if childError.Error() != expected {
		t.Fatalf("expected %q but received error message %q", expected, childError.Error())
	}
}

func TestSendPayloadToChild(t *testing.T) {
	pipe, err := NewSyncPipe()
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := pipe.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	expected := "libcontainer"

	if err := pipe.SendToChild(testStruct{Name: expected}); err != nil {
		t.Fatal(err)
	}

	var s *testStruct
	if err := pipe.ReadFromParent(&s); err != nil {
		t.Fatal(err)
	}

	if s.Name != expected {
		t.Fatalf("expected name %q but received %q", expected, s.Name)
	}
}
