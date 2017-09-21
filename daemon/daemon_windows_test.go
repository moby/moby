// +build windows

package daemon

import (
	"strings"
	"testing"

	"golang.org/x/sys/windows/svc/mgr"
)

const existingService = "Power"

func TestEnsureServicesExist(t *testing.T) {
	m, err := mgr.Connect()
	if err != nil {
		t.Fatal("failed to connect to service manager, this test needs admin")
	}
	defer m.Disconnect()
	s, err := m.OpenService(existingService)
	if err != nil {
		t.Fatalf("expected to find known inbox service %q, this test needs a known inbox service to run correctly", existingService)
	}
	defer s.Close()

	input := []string{existingService}
	err = ensureServicesInstalled(input)
	if err != nil {
		t.Fatalf("unexpected error for input %q: %q", input, err)
	}
}

func TestEnsureServicesExistErrors(t *testing.T) {
	m, err := mgr.Connect()
	if err != nil {
		t.Fatal("failed to connect to service manager, this test needs admin")
	}
	defer m.Disconnect()
	s, err := m.OpenService(existingService)
	if err != nil {
		t.Fatalf("expected to find known inbox service %q, this test needs a known inbox service to run correctly", existingService)
	}
	defer s.Close()

	for _, testcase := range []struct {
		input         []string
		expectedError string
	}{
		{
			input:         []string{"daemon_windows_test_fakeservice"},
			expectedError: "failed to open service daemon_windows_test_fakeservice",
		},
		{
			input:         []string{"daemon_windows_test_fakeservice1", "daemon_windows_test_fakeservice2"},
			expectedError: "failed to open service daemon_windows_test_fakeservice1",
		},
		{
			input:         []string{existingService, "daemon_windows_test_fakeservice"},
			expectedError: "failed to open service daemon_windows_test_fakeservice",
		},
	} {
		t.Run(strings.Join(testcase.input, ";"), func(t *testing.T) {
			err := ensureServicesInstalled(testcase.input)
			if err == nil {
				t.Fatalf("expected error for input %v", testcase.input)
			}
			if !strings.Contains(err.Error(), testcase.expectedError) {
				t.Fatalf("expected error %q to contain %q", err.Error(), testcase.expectedError)
			}
		})
	}
}
