package sandbox

import "testing"

func TestSandboxCreate(t *testing.T) {
	key, err := newKey(t)
	if err != nil {
		t.Fatalf("Failed to obtain a key: %v", err)
	}

	s, err := NewSandbox(key)
	if err != nil {
		t.Fatalf("Failed to create a new sandbox: %v", err)
	}

	if s.Key() != key {
		t.Fatalf("s.Key() returned %s. Expected %s", s.Key(), key)
	}

	verifySandbox(t, s)
}
