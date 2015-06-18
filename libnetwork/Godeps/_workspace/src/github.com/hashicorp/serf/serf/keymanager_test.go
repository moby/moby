package serf

import (
	"bytes"
	"encoding/base64"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/testutil"
	"testing"
)

func testKeyring() (*memberlist.Keyring, error) {
	keys := []string{
		"enjTwAFRe4IE71bOFhirzQ==",
		"csT9mxI7aTf9ap3HLBbdmA==",
		"noha2tVc0OyD/2LtCBoAOQ==",
	}

	keysDecoded := make([][]byte, len(keys))
	for i, key := range keys {
		decoded, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			return nil, err
		}
		keysDecoded[i] = decoded
	}

	return memberlist.NewKeyring(keysDecoded, keysDecoded[0])
}

func testKeyringSerf() (*Serf, error) {
	config := testConfig()

	keyring, err := testKeyring()
	if err != nil {
		return nil, err
	}
	config.MemberlistConfig.Keyring = keyring

	s, err := Create(config)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func keyExistsInRing(kr *memberlist.Keyring, key []byte) bool {
	for _, installedKey := range kr.GetKeys() {
		if bytes.Equal(key, installedKey) {
			return true
		}
	}
	return false
}

func TestSerf_InstallKey(t *testing.T) {
	s1, err := testKeyringSerf()
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer s1.Shutdown()

	s2, err := testKeyringSerf()
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer s2.Shutdown()

	primaryKey := s1.config.MemberlistConfig.Keyring.GetPrimaryKey()

	// Join s1 and s2
	_, err = s1.Join([]string{s2.config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Begin tests
	newKey := "l4ZkaypGLT8AsB0LBldthw=="
	newKeyBytes, err := base64.StdEncoding.DecodeString(newKey)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	manager := s1.KeyManager()

	// Install a new key onto the existing ring. This is a blocking call, so no
	// need for a yield.
	_, err = manager.InstallKey(newKey)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Key installation did not affect the current primary key
	if !bytes.Equal(primaryKey, s1.config.MemberlistConfig.Keyring.GetPrimaryKey()) {
		t.Fatal("Unexpected primary key change on s1")
	}

	if !bytes.Equal(primaryKey, s2.config.MemberlistConfig.Keyring.GetPrimaryKey()) {
		t.Fatal("Unexpected primary key change on s2")
	}

	// New key was successfully broadcasted and installed on all members
	if !keyExistsInRing(s1.config.MemberlistConfig.Keyring, newKeyBytes) {
		t.Fatal("Newly-installed key not found in keyring on s1")
	}

	if !keyExistsInRing(s2.config.MemberlistConfig.Keyring, newKeyBytes) {
		t.Fatal("Newly-installed key not found in keyring on s2")
	}
}

func TestSerf_UseKey(t *testing.T) {
	s1, err := testKeyringSerf()
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer s1.Shutdown()

	s2, err := testKeyringSerf()
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer s2.Shutdown()

	// Join s1 and s2
	_, err = s1.Join([]string{s2.config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Begin tests
	useKey := "csT9mxI7aTf9ap3HLBbdmA=="
	useKeyBytes, err := base64.StdEncoding.DecodeString(useKey)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	manager := s1.KeyManager()

	// Change the primary encryption key
	_, err = manager.UseKey(useKey)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// First make sure that the primary key is what we expect it to be
	if !bytes.Equal(useKeyBytes, s1.config.MemberlistConfig.Keyring.GetPrimaryKey()) {
		t.Fatal("Unexpected primary key on s1")
	}

	if !bytes.Equal(useKeyBytes, s2.config.MemberlistConfig.Keyring.GetPrimaryKey()) {
		t.Fatal("Unexpected primary key on s2")
	}

	// Make sure an error is thrown if the key doesn't exist
	_, err = manager.UseKey("aE6AfGEvay+UJbkfxBk4SQ==")
	if err == nil {
		t.Fatalf("Expected error changing to non-existent primary key")
	}
}

func TestSerf_RemoveKey(t *testing.T) {
	s1, err := testKeyringSerf()
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer s1.Shutdown()

	s2, err := testKeyringSerf()
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer s2.Shutdown()

	// Join s1 and s2
	_, err = s1.Join([]string{s2.config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	// Begin tests
	removeKey := "noha2tVc0OyD/2LtCBoAOQ=="
	removeKeyBytes, err := base64.StdEncoding.DecodeString(removeKey)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	manager := s1.KeyManager()

	// Remove a key from the ring
	_, err = manager.RemoveKey(removeKey)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Key was successfully removed from all members
	if keyExistsInRing(s1.config.MemberlistConfig.Keyring, removeKeyBytes) {
		t.Fatal("Key not removed from keyring on s1")
	}

	if keyExistsInRing(s2.config.MemberlistConfig.Keyring, removeKeyBytes) {
		t.Fatal("Key not removed from keyring on s2")
	}
}

func TestSerf_ListKeys(t *testing.T) {
	s1, err := testKeyringSerf()
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer s1.Shutdown()

	s2, err := testKeyringSerf()
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer s2.Shutdown()

	manager := s1.KeyManager()

	initialKeyringLen := len(s1.config.MemberlistConfig.Keyring.GetKeys())

	// Extra key on s2 to make sure we see it in the list
	extraKey := "JHAxGU13qDaXhUW6jIpyog=="
	extraKeyBytes, err := base64.StdEncoding.DecodeString(extraKey)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	s2.config.MemberlistConfig.Keyring.AddKey(extraKeyBytes)

	// Join s1 and s2
	_, err = s1.Join([]string{s2.config.MemberlistConfig.BindAddr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	resp, err := manager.ListKeys()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Found all keys in the list
	expected := initialKeyringLen + 1
	if expected != len(resp.Keys) {
		t.Fatalf("Expected %d keys in result, found %d", expected, len(resp.Keys))
	}

	found := false
	for key, _ := range resp.Keys {
		if key == extraKey {
			found = true
		}
	}
	if !found {
		t.Fatalf("Did not find expected key in list: %s", extraKey)
	}

	// Number of members with extra key installed should be 1
	for key, num := range resp.Keys {
		if key == extraKey && num != 1 {
			t.Fatalf("Expected 1 nodes with key %s but have %d", extraKey, num)
		}
	}
}
