package layer

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/moby/moby/v2/daemon/internal/stringid"
	"github.com/opencontainers/go-digest"
)

func randomLayerID(seed int64) ChainID {
	r := rand.New(rand.NewSource(seed)).Int63()
	return digest.FromBytes([]byte(strconv.FormatInt(r, 10)))
}

func newFileMetadataStore(t *testing.T) (*fileMetadataStore, string) {
	t.Helper()
	td := t.TempDir()
	fms, err := newFSMetadataStore(td)
	if err != nil {
		t.Fatal(err)
	}

	return fms, td
}

func TestCommitFailure(t *testing.T) {
	fms, td := newFileMetadataStore(t)

	if err := os.WriteFile(filepath.Join(td, "sha256"), []byte("was here first!"), 0o644); err != nil {
		t.Fatal(err)
	}

	tx, err := fms.StartTransaction()
	if err != nil {
		t.Fatal(err)
	}

	if err := tx.SetSize(0); err != nil {
		t.Fatal(err)
	}

	err = tx.Commit(randomLayerID(5))
	if err == nil {
		t.Fatalf("Expected error committing with invalid layer parent directory")
	}
	if !errors.Is(err, syscall.ENOTDIR) {
		t.Errorf("Unexpected error %s (%[1]T), expected %s", err, syscall.ENOTDIR)
	}
}

func TestStartTransactionFailure(t *testing.T) {
	fms, td := newFileMetadataStore(t)

	if err := os.WriteFile(filepath.Join(td, "tmp"), []byte("was here first!"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := fms.StartTransaction()
	if err == nil {
		t.Fatalf("Expected error starting transaction with invalid layer parent directory")
	}
	if !errors.Is(err, syscall.ENOTDIR) {
		t.Errorf("Unexpected error %s (%[1]T), expected %s", err, syscall.ENOTDIR)
	}

	if err := os.Remove(filepath.Join(td, "tmp")); err != nil {
		t.Fatal(err)
	}

	tx, err := fms.StartTransaction()
	if err != nil {
		t.Fatal(err)
	}

	if expected := filepath.Join(td, "tmp"); strings.HasPrefix(expected, tx.String()) {
		t.Fatalf("Unexpected transaction string %q, expected prefix %q", tx.String(), expected)
	}

	if err := tx.Cancel(); err != nil {
		t.Fatal(err)
	}
}

func TestGetOrphan(t *testing.T) {
	fms, td := newFileMetadataStore(t)

	layerRoot := filepath.Join(td, "sha256")
	if err := os.MkdirAll(layerRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	tx, err := fms.StartTransaction()
	if err != nil {
		t.Fatal(err)
	}

	layerID := randomLayerID(5)
	err = tx.Commit(layerID)
	if err != nil {
		t.Fatal(err)
	}
	layerPath := fms.getLayerDirectory(layerID)
	if err := os.WriteFile(filepath.Join(layerPath, "cache-id"), []byte(stringid.GenerateRandomID()), 0o644); err != nil {
		t.Fatal(err)
	}

	orphanLayers, err := fms.getOrphan()
	if err != nil {
		t.Fatal(err)
	}
	if len(orphanLayers) != 0 {
		t.Fatalf("Expected to have zero orphan layers")
	}

	_, idValue, _ := strings.Cut(layerID.String(), ":")
	newPath := filepath.Join(layerRoot, fmt.Sprintf("%s-%s-removing", idValue, stringid.GenerateRandomID()))
	err = os.Rename(layerPath, newPath)
	if err != nil {
		t.Fatal(err)
	}
	orphanLayers, err = fms.getOrphan()
	if err != nil {
		t.Fatal(err)
	}
	if len(orphanLayers) != 1 {
		t.Fatalf("Expected to have one orphan layer")
	}
}

func TestIsValidID(t *testing.T) {
	testCases := []struct {
		name     string
		id       string
		expected bool
	}{
		{"Valid 64-char hexadecimal", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", true},
		{"Valid 64-char hexadecimal with -init suffix", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef-init", true},
		{"Invalid: too short", "1234567890abcdef", false},
		{"Invalid: too long", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef00", false},
		{"Invalid: contains uppercase letter", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdeF", false},
		{"Invalid: contains non-hexadecimal character", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdeg", false},
		{"Invalid: empty string", "", false},
		{"Invalid: only -init suffix", "-init", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isValidID(tc.id)
			if result != tc.expected {
				t.Errorf("isValidID(%q): got %v, want %v", tc.id, result, tc.expected)
			}
		})
	}
}
