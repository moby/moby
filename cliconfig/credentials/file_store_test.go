package credentials

import (
	"io/ioutil"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/cliconfig/configfile"
)

func newConfigFile(auths map[string]types.AuthConfig) *configfile.ConfigFile {
	tmp, _ := ioutil.TempFile("", "docker-test")
	name := tmp.Name()
	tmp.Close()

	c := cliconfig.NewConfigFile(name)
	c.AuthConfigs = auths
	return c
}

func TestFileStoreAddCredentials(t *testing.T) {
	f := newConfigFile(make(map[string]types.AuthConfig))

	s := NewFileStore(f)
	err := s.Store(types.AuthConfig{
		Auth:          "super_secret_token",
		Email:         "foo@example.com",
		ServerAddress: "https://example.com",
	})

	if err != nil {
		t.Fatal(err)
	}

	if len(f.AuthConfigs) != 1 {
		t.Fatalf("expected 1 auth config, got %d", len(f.AuthConfigs))
	}

	a, ok := f.AuthConfigs["https://example.com"]
	if !ok {
		t.Fatalf("expected auth for https://example.com, got %v", f.AuthConfigs)
	}
	if a.Auth != "super_secret_token" {
		t.Fatalf("expected auth `super_secret_token`, got %s", a.Auth)
	}
	if a.Email != "foo@example.com" {
		t.Fatalf("expected email `foo@example.com`, got %s", a.Email)
	}
}

func TestFileStoreGet(t *testing.T) {
	f := newConfigFile(map[string]types.AuthConfig{
		"https://example.com": {
			Auth:          "super_secret_token",
			Email:         "foo@example.com",
			ServerAddress: "https://example.com",
		},
	})

	s := NewFileStore(f)
	a, err := s.Get("https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if a.Auth != "super_secret_token" {
		t.Fatalf("expected auth `super_secret_token`, got %s", a.Auth)
	}
	if a.Email != "foo@example.com" {
		t.Fatalf("expected email `foo@example.com`, got %s", a.Email)
	}
}

func TestFileStoreGetAll(t *testing.T) {
	s1 := "https://example.com"
	s2 := "https://example2.com"
	f := newConfigFile(map[string]types.AuthConfig{
		s1: {
			Auth:          "super_secret_token",
			Email:         "foo@example.com",
			ServerAddress: "https://example.com",
		},
		s2: {
			Auth:          "super_secret_token2",
			Email:         "foo@example2.com",
			ServerAddress: "https://example2.com",
		},
	})

	s := NewFileStore(f)
	as, err := s.GetAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(as) != 2 {
		t.Fatalf("wanted 2, got %d", len(as))
	}
	if as[s1].Auth != "super_secret_token" {
		t.Fatalf("expected auth `super_secret_token`, got %s", as[s1].Auth)
	}
	if as[s1].Email != "foo@example.com" {
		t.Fatalf("expected email `foo@example.com`, got %s", as[s1].Email)
	}
	if as[s2].Auth != "super_secret_token2" {
		t.Fatalf("expected auth `super_secret_token2`, got %s", as[s2].Auth)
	}
	if as[s2].Email != "foo@example2.com" {
		t.Fatalf("expected email `foo@example2.com`, got %s", as[s2].Email)
	}
}

func TestFileStoreErase(t *testing.T) {
	f := newConfigFile(map[string]types.AuthConfig{
		"https://example.com": {
			Auth:          "super_secret_token",
			Email:         "foo@example.com",
			ServerAddress: "https://example.com",
		},
	})

	s := NewFileStore(f)
	err := s.Erase("https://example.com")
	if err != nil {
		t.Fatal(err)
	}

	// file store never returns errors, check that the auth config is empty
	a, err := s.Get("https://example.com")
	if err != nil {
		t.Fatal(err)
	}

	if a.Auth != "" {
		t.Fatalf("expected empty auth token, got %s", a.Auth)
	}
	if a.Email != "" {
		t.Fatalf("expected empty email, got %s", a.Email)
	}
}
