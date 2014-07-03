package namespaces

import (
	"fmt"
	"testing"

	"github.com/docker/libcontainer/network"
)

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

	expected := "something bad happened"

	pipe.ReportChildError(fmt.Errorf(expected))

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

	if err := pipe.SendToChild(&network.NetworkState{VethHost: expected}); err != nil {
		t.Fatal(err)
	}

	payload, err := pipe.ReadFromParent()
	if err != nil {
		t.Fatal(err)
	}

	if payload.VethHost != expected {
		t.Fatalf("expected veth host %q but received %q", expected, payload.VethHost)
	}
}
