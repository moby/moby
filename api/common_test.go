package api

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"os"

	"github.com/docker/docker/api/types"
)

type ports struct {
	ports    []types.Port
	expected string
}

// DisplayablePorts
func TestDisplayablePorts(t *testing.T) {
	cases := []ports{
		{
			[]types.Port{
				{
					PrivatePort: 9988,
					Type:        "tcp",
				},
			},
			"9988/tcp"},
		{
			[]types.Port{
				{
					PrivatePort: 9988,
					Type:        "udp",
				},
			},
			"9988/udp",
		},
		{
			[]types.Port{
				{
					IP:          "0.0.0.0",
					PrivatePort: 9988,
					Type:        "tcp",
				},
			},
			"0.0.0.0:0->9988/tcp",
		},
		{
			[]types.Port{
				{
					PrivatePort: 9988,
					PublicPort:  8899,
					Type:        "tcp",
				},
			},
			"9988/tcp",
		},
		{
			[]types.Port{
				{
					IP:          "4.3.2.1",
					PrivatePort: 9988,
					PublicPort:  8899,
					Type:        "tcp",
				},
			},
			"4.3.2.1:8899->9988/tcp",
		},
		{
			[]types.Port{
				{
					IP:          "4.3.2.1",
					PrivatePort: 9988,
					PublicPort:  9988,
					Type:        "tcp",
				},
			},
			"4.3.2.1:9988->9988/tcp",
		},
		{
			[]types.Port{
				{
					PrivatePort: 9988,
					Type:        "udp",
				}, {
					PrivatePort: 9988,
					Type:        "udp",
				},
			},
			"9988/udp, 9988/udp",
		},
		{
			[]types.Port{
				{
					IP:          "1.2.3.4",
					PublicPort:  9998,
					PrivatePort: 9998,
					Type:        "udp",
				}, {
					IP:          "1.2.3.4",
					PublicPort:  9999,
					PrivatePort: 9999,
					Type:        "udp",
				},
			},
			"1.2.3.4:9998-9999->9998-9999/udp",
		},
		{
			[]types.Port{
				{
					IP:          "1.2.3.4",
					PublicPort:  8887,
					PrivatePort: 9998,
					Type:        "udp",
				}, {
					IP:          "1.2.3.4",
					PublicPort:  8888,
					PrivatePort: 9999,
					Type:        "udp",
				},
			},
			"1.2.3.4:8887->9998/udp, 1.2.3.4:8888->9999/udp",
		},
		{
			[]types.Port{
				{
					PrivatePort: 9998,
					Type:        "udp",
				}, {
					PrivatePort: 9999,
					Type:        "udp",
				},
			},
			"9998-9999/udp",
		},
		{
			[]types.Port{
				{
					IP:          "1.2.3.4",
					PrivatePort: 6677,
					PublicPort:  7766,
					Type:        "tcp",
				}, {
					PrivatePort: 9988,
					PublicPort:  8899,
					Type:        "udp",
				},
			},
			"9988/udp, 1.2.3.4:7766->6677/tcp",
		},
		{
			[]types.Port{
				{
					IP:          "1.2.3.4",
					PrivatePort: 9988,
					PublicPort:  8899,
					Type:        "udp",
				}, {
					IP:          "1.2.3.4",
					PrivatePort: 9988,
					PublicPort:  8899,
					Type:        "tcp",
				}, {
					IP:          "4.3.2.1",
					PrivatePort: 2233,
					PublicPort:  3322,
					Type:        "tcp",
				},
			},
			"4.3.2.1:3322->2233/tcp, 1.2.3.4:8899->9988/tcp, 1.2.3.4:8899->9988/udp",
		},
		{
			[]types.Port{
				{
					PrivatePort: 9988,
					PublicPort:  8899,
					Type:        "udp",
				}, {
					IP:          "1.2.3.4",
					PrivatePort: 6677,
					PublicPort:  7766,
					Type:        "tcp",
				}, {
					IP:          "4.3.2.1",
					PrivatePort: 2233,
					PublicPort:  3322,
					Type:        "tcp",
				},
			},
			"9988/udp, 4.3.2.1:3322->2233/tcp, 1.2.3.4:7766->6677/tcp",
		},
		{
			[]types.Port{
				{
					PrivatePort: 80,
					Type:        "tcp",
				}, {
					PrivatePort: 1024,
					Type:        "tcp",
				}, {
					PrivatePort: 80,
					Type:        "udp",
				}, {
					PrivatePort: 1024,
					Type:        "udp",
				}, {
					IP:          "1.1.1.1",
					PublicPort:  80,
					PrivatePort: 1024,
					Type:        "tcp",
				}, {
					IP:          "1.1.1.1",
					PublicPort:  80,
					PrivatePort: 1024,
					Type:        "udp",
				}, {
					IP:          "1.1.1.1",
					PublicPort:  1024,
					PrivatePort: 80,
					Type:        "tcp",
				}, {
					IP:          "1.1.1.1",
					PublicPort:  1024,
					PrivatePort: 80,
					Type:        "udp",
				}, {
					IP:          "2.1.1.1",
					PublicPort:  80,
					PrivatePort: 1024,
					Type:        "tcp",
				}, {
					IP:          "2.1.1.1",
					PublicPort:  80,
					PrivatePort: 1024,
					Type:        "udp",
				}, {
					IP:          "2.1.1.1",
					PublicPort:  1024,
					PrivatePort: 80,
					Type:        "tcp",
				}, {
					IP:          "2.1.1.1",
					PublicPort:  1024,
					PrivatePort: 80,
					Type:        "udp",
				},
			},
			"80/tcp, 80/udp, 1024/tcp, 1024/udp, 1.1.1.1:1024->80/tcp, 1.1.1.1:1024->80/udp, 2.1.1.1:1024->80/tcp, 2.1.1.1:1024->80/udp, 1.1.1.1:80->1024/tcp, 1.1.1.1:80->1024/udp, 2.1.1.1:80->1024/tcp, 2.1.1.1:80->1024/udp",
		},
	}

	for _, port := range cases {
		actual := DisplayablePorts(port.ports)
		if port.expected != actual {
			t.Fatalf("Expected %s, got %s.", port.expected, actual)
		}
	}
}

// MatchesContentType
func TestJsonContentType(t *testing.T) {
	if !MatchesContentType("application/json", "application/json") {
		t.Fail()
	}

	if !MatchesContentType("application/json; charset=utf-8", "application/json") {
		t.Fail()
	}

	if MatchesContentType("dockerapplication/json", "application/json") {
		t.Fail()
	}
}

// LoadOrCreateTrustKey
func TestLoadOrCreateTrustKeyInvalidKeyFile(t *testing.T) {
	tmpKeyFolderPath, err := ioutil.TempDir("", "api-trustkey-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpKeyFolderPath)

	tmpKeyFile, err := ioutil.TempFile(tmpKeyFolderPath, "keyfile")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := LoadOrCreateTrustKey(tmpKeyFile.Name()); err == nil {
		t.Fatalf("expected an error, got nothing.")
	}

}

func TestLoadOrCreateTrustKeyCreateKey(t *testing.T) {
	tmpKeyFolderPath, err := ioutil.TempDir("", "api-trustkey-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpKeyFolderPath)

	// Without the need to create the folder hierarchy
	tmpKeyFile := filepath.Join(tmpKeyFolderPath, "keyfile")

	if key, err := LoadOrCreateTrustKey(tmpKeyFile); err != nil || key == nil {
		t.Fatalf("expected a new key file, got : %v and %v", err, key)
	}

	if _, err := os.Stat(tmpKeyFile); err != nil {
		t.Fatalf("Expected to find a file %s, got %v", tmpKeyFile, err)
	}

	// With the need to create the folder hierarchy as tmpKeyFie is in a path
	// where some folders do not exist.
	tmpKeyFile = filepath.Join(tmpKeyFolderPath, "folder/hierarchy/keyfile")

	if key, err := LoadOrCreateTrustKey(tmpKeyFile); err != nil || key == nil {
		t.Fatalf("expected a new key file, got : %v and %v", err, key)
	}

	if _, err := os.Stat(tmpKeyFile); err != nil {
		t.Fatalf("Expected to find a file %s, got %v", tmpKeyFile, err)
	}

	// With no path at all
	defer os.Remove("keyfile")
	if key, err := LoadOrCreateTrustKey("keyfile"); err != nil || key == nil {
		t.Fatalf("expected a new key file, got : %v and %v", err, key)
	}

	if _, err := os.Stat("keyfile"); err != nil {
		t.Fatalf("Expected to find a file keyfile, got %v", err)
	}
}

func TestLoadOrCreateTrustKeyLoadValidKey(t *testing.T) {
	tmpKeyFile := filepath.Join("fixtures", "keyfile")

	if key, err := LoadOrCreateTrustKey(tmpKeyFile); err != nil || key == nil {
		t.Fatalf("expected a key file, got : %v and %v", err, key)
	}
}
