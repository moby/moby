package credentials

import (
	"io/ioutil"
	"testing"

	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/cliconfig/configfile"
	"github.com/docker/engine-api/types"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func newConfigFile(auths map[string]types.AuthConfig) *configfile.ConfigFile {
	tmp, _ := ioutil.TempFile("", "docker-test")
	name := tmp.Name()
	tmp.Close()

	c := cliconfig.NewConfigFile(name)
	c.AuthConfigs = auths
	return c
}

func (s *DockerSuite) TestFileStoreAddCredentials(c *check.C) {
	f := newConfigFile(make(map[string]types.AuthConfig))

	fs := NewFileStore(f)
	err := fs.Store(types.AuthConfig{
		Auth:          "super_secret_token",
		Email:         "foo@example.com",
		ServerAddress: "https://example.com",
	})

	if err != nil {
		c.Fatal(err)
	}

	if len(f.AuthConfigs) != 1 {
		c.Fatalf("expected 1 auth config, got %d", len(f.AuthConfigs))
	}

	a, ok := f.AuthConfigs["https://example.com"]
	if !ok {
		c.Fatalf("expected auth for https://example.com, got %v", f.AuthConfigs)
	}
	if a.Auth != "super_secret_token" {
		c.Fatalf("expected auth `super_secret_token`, got %s", a.Auth)
	}
	if a.Email != "foo@example.com" {
		c.Fatalf("expected email `foo@example.com`, got %s", a.Email)
	}
}

func (s *DockerSuite) TestFileStoreGet(c *check.C) {
	f := newConfigFile(map[string]types.AuthConfig{
		"https://example.com": {
			Auth:          "super_secret_token",
			Email:         "foo@example.com",
			ServerAddress: "https://example.com",
		},
	})

	fs := NewFileStore(f)
	a, err := fs.Get("https://example.com")
	if err != nil {
		c.Fatal(err)
	}
	if a.Auth != "super_secret_token" {
		c.Fatalf("expected auth `super_secret_token`, got %s", a.Auth)
	}
	if a.Email != "foo@example.com" {
		c.Fatalf("expected email `foo@example.com`, got %s", a.Email)
	}
}

func (s *DockerSuite) TestFileStoreGetAll(c *check.C) {
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

	fs := NewFileStore(f)
	as, err := fs.GetAll()
	if err != nil {
		c.Fatal(err)
	}
	if len(as) != 2 {
		c.Fatalf("wanted 2, got %d", len(as))
	}
	if as[s1].Auth != "super_secret_token" {
		c.Fatalf("expected auth `super_secret_token`, got %s", as[s1].Auth)
	}
	if as[s1].Email != "foo@example.com" {
		c.Fatalf("expected email `foo@example.com`, got %s", as[s1].Email)
	}
	if as[s2].Auth != "super_secret_token2" {
		c.Fatalf("expected auth `super_secret_token2`, got %s", as[s2].Auth)
	}
	if as[s2].Email != "foo@example2.com" {
		c.Fatalf("expected email `foo@example2.com`, got %s", as[s2].Email)
	}
}

func (s *DockerSuite) TestFileStoreErase(c *check.C) {
	f := newConfigFile(map[string]types.AuthConfig{
		"https://example.com": {
			Auth:          "super_secret_token",
			Email:         "foo@example.com",
			ServerAddress: "https://example.com",
		},
	})

	fs := NewFileStore(f)
	err := fs.Erase("https://example.com")
	if err != nil {
		c.Fatal(err)
	}

	// file store never returns errors, check that the auth config is empty
	a, err := fs.Get("https://example.com")
	if err != nil {
		c.Fatal(err)
	}

	if a.Auth != "" {
		c.Fatalf("expected empty auth token, got %s", a.Auth)
	}
	if a.Email != "" {
		c.Fatalf("expected empty email, got %s", a.Email)
	}
}
