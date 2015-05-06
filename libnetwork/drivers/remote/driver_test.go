package remote

import (
	"testing"

	"github.com/docker/libnetwork/driverapi"
)

type testCallbackStruct struct {
	networkType string
}

func (t *testCallbackStruct) RegisterDriver(networkType string, driver driverapi.Driver) error {
	t.networkType = networkType
	return nil
}

func TestCallback(t *testing.T) {
	tc := &testCallbackStruct{}
	_, d := New(tc)
	expected := "test-dummy"
	_, err := d.(*driver).registerRemoteDriver(expected)
	if err != nil {
		t.Fatalf("Remote Driver callback registration failed with Error : %v", err)
	}
	if tc.networkType != expected {
		t.Fatal("Remote Driver Callback Registration failed")
	}
}
