package homedir // import "github.com/docker/docker/pkg/homedir"

import (
	"path/filepath"
	"testing"
)

func TestGet(t *testing.T) {
	home := Get()
	if home == "" {
		t.Fatal("returned home directory is empty")
	}

	if !filepath.IsAbs(home) {
		t.Fatalf("returned path is not absolute: %s", home)
	}
}

func TestGetShortcutString(t *testing.T) {
	shortcut := GetShortcutString() //nolint:staticcheck // ignore SA1019 (GetShortcutString is deprecated)
	if shortcut == "" {
		t.Fatal("returned shortcut string is empty")
	}
}
