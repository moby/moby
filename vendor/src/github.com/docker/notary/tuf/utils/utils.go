package utils

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/docker/notary/tuf/data"
)

// Download does a simple download from a URL
func Download(url url.URL) (*http.Response, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	return client.Get(url.String())
}

// Upload does a simple JSON upload to a URL
func Upload(url string, body io.Reader) (*http.Response, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	return client.Post(url, "application/json", body)
}

// ValidateTarget ensures that the data read from reader matches
// the known metadata
func ValidateTarget(r io.Reader, m *data.FileMeta) error {
	h := sha256.New()
	length, err := io.Copy(h, r)
	if err != nil {
		return err
	}
	if length != m.Length {
		return fmt.Errorf("Size of downloaded target did not match targets entry.\nExpected: %d\nReceived: %d\n", m.Length, length)
	}
	hashDigest := h.Sum(nil)
	if bytes.Compare(m.Hashes["sha256"], hashDigest[:]) != 0 {
		return fmt.Errorf("Hash of downloaded target did not match targets entry.\nExpected: %x\nReceived: %x\n", m.Hashes["sha256"], hashDigest)
	}
	return nil
}

// StrSliceContains checks if the given string appears in the slice
func StrSliceContains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// StrSliceContainsI checks if the given string appears in the slice
// in a case insensitive manner
func StrSliceContainsI(ss []string, s string) bool {
	s = strings.ToLower(s)
	for _, v := range ss {
		v = strings.ToLower(v)
		if v == s {
			return true
		}
	}
	return false
}

// FileExists returns true if a file (or dir) exists at the given path,
// false otherwise
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

// NoopCloser is a simple Reader wrapper that does nothing when Close is
// called
type NoopCloser struct {
	io.Reader
}

// Close does nothing for a NoopCloser
func (nc *NoopCloser) Close() error {
	return nil
}

// DoHash returns the digest of d using the hashing algorithm named
// in alg
func DoHash(alg string, d []byte) []byte {
	switch alg {
	case "sha256":
		digest := sha256.Sum256(d)
		return digest[:]
	case "sha512":
		digest := sha512.Sum512(d)
		return digest[:]
	}
	return nil
}

// UnusedDelegationKeys prunes a list of keys, returning those that are no
// longer in use for a given targets file
func UnusedDelegationKeys(t data.SignedTargets) []string {
	// compare ids to all still active key ids in all active roles
	// with the targets file
	found := make(map[string]bool)
	for _, r := range t.Signed.Delegations.Roles {
		for _, id := range r.KeyIDs {
			found[id] = true
		}
	}
	var discard []string
	for id := range t.Signed.Delegations.Keys {
		if !found[id] {
			discard = append(discard, id)
		}
	}
	return discard
}

// RemoveUnusedKeys determines which keys in the slice of IDs are no longer
// used in the given targets file and removes them from the delegated keys
// map
func RemoveUnusedKeys(t *data.SignedTargets) {
	unusedIDs := UnusedDelegationKeys(*t)
	for _, id := range unusedIDs {
		delete(t.Signed.Delegations.Keys, id)
	}
}

// FindRoleIndex returns the index of the role named <name> or -1 if no
// matching role is found.
func FindRoleIndex(rs []*data.Role, name string) int {
	for i, r := range rs {
		if r.Name == name {
			return i
		}
	}
	return -1
}
