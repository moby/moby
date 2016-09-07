package credentials

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/docker/docker-credential-helpers/client"
	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/docker/engine-api/types"
	"github.com/go-check/check"
)

const (
	validServerAddress   = "https://index.docker.io/v1"
	validServerAddress2  = "https://example.com:5002"
	invalidServerAddress = "https://foobar.example.com"
	missingCredsAddress  = "https://missing.docker.io/v1"
)

var errCommandExited = fmt.Errorf("exited 1")

// mockCommand simulates interactions between the docker client and a remote
// credentials helper.
// Unit tests inject this mocked command into the remote to control execution.
type mockCommand struct {
	arg   string
	input io.Reader
}

// Output returns responses from the remote credentials helper.
// It mocks those responses based in the input in the mock.
func (m *mockCommand) Output() ([]byte, error) {
	in, err := ioutil.ReadAll(m.input)
	if err != nil {
		return nil, err
	}
	inS := string(in)

	switch m.arg {
	case "erase":
		switch inS {
		case validServerAddress:
			return nil, nil
		default:
			return []byte("program failed"), errCommandExited
		}
	case "get":
		switch inS {
		case validServerAddress:
			return []byte(`{"Username": "foo", "Secret": "bar"}`), nil
		case validServerAddress2:
			return []byte(`{"Username": "<token>", "Secret": "abcd1234"}`), nil
		case missingCredsAddress:
			return []byte(credentials.NewErrCredentialsNotFound().Error()), errCommandExited
		case invalidServerAddress:
			return []byte("program failed"), errCommandExited
		}
	case "store":
		var c credentials.Credentials
		err := json.NewDecoder(strings.NewReader(inS)).Decode(&c)
		if err != nil {
			return []byte("program failed"), errCommandExited
		}
		switch c.ServerURL {
		case validServerAddress:
			return nil, nil
		default:
			return []byte("program failed"), errCommandExited
		}
	}

	return []byte(fmt.Sprintf("unknown argument %q with %q", m.arg, inS)), errCommandExited
}

// Input sets the input to send to a remote credentials helper.
func (m *mockCommand) Input(in io.Reader) {
	m.input = in
}

func mockCommandFn(args ...string) client.Program {
	return &mockCommand{
		arg: args[0],
	}
}

func (s *DockerSuite) TestNativeStoreAddCredentials(c *check.C) {
	f := newConfigFile(make(map[string]types.AuthConfig))
	f.CredentialsStore = "mock"

	ns := &nativeStore{
		programFunc: mockCommandFn,
		fileStore:   NewFileStore(f),
	}
	err := ns.Store(types.AuthConfig{
		Username:      "foo",
		Password:      "bar",
		Email:         "foo@example.com",
		ServerAddress: validServerAddress,
	})

	if err != nil {
		c.Fatal(err)
	}

	if len(f.AuthConfigs) != 1 {
		c.Fatalf("expected 1 auth config, got %d", len(f.AuthConfigs))
	}

	a, ok := f.AuthConfigs[validServerAddress]
	if !ok {
		c.Fatalf("expected auth for %s, got %v", validServerAddress, f.AuthConfigs)
	}
	if a.Auth != "" {
		c.Fatalf("expected auth to be empty, got %s", a.Auth)
	}
	if a.Username != "" {
		c.Fatalf("expected username to be empty, got %s", a.Username)
	}
	if a.Password != "" {
		c.Fatalf("expected password to be empty, got %s", a.Password)
	}
	if a.IdentityToken != "" {
		c.Fatalf("expected identity token to be empty, got %s", a.IdentityToken)
	}
	if a.Email != "foo@example.com" {
		c.Fatalf("expected email `foo@example.com`, got %s", a.Email)
	}
}

func (s *DockerSuite) TestNativeStoreAddInvalidCredentials(c *check.C) {
	f := newConfigFile(make(map[string]types.AuthConfig))
	f.CredentialsStore = "mock"

	ns := &nativeStore{
		programFunc: mockCommandFn,
		fileStore:   NewFileStore(f),
	}
	err := ns.Store(types.AuthConfig{
		Username:      "foo",
		Password:      "bar",
		Email:         "foo@example.com",
		ServerAddress: invalidServerAddress,
	})

	if err == nil {
		c.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "program failed") {
		c.Fatalf("expected `program failed`, got %v", err)
	}

	if len(f.AuthConfigs) != 0 {
		c.Fatalf("expected 0 auth config, got %d", len(f.AuthConfigs))
	}
}

func (s *DockerSuite) TestNativeStoreGet(c *check.C) {
	f := newConfigFile(map[string]types.AuthConfig{
		validServerAddress: {
			Email: "foo@example.com",
		},
	})
	f.CredentialsStore = "mock"

	ns := &nativeStore{
		programFunc: mockCommandFn,
		fileStore:   NewFileStore(f),
	}
	a, err := ns.Get(validServerAddress)
	if err != nil {
		c.Fatal(err)
	}

	if a.Username != "foo" {
		c.Fatalf("expected username `foo`, got %s", a.Username)
	}
	if a.Password != "bar" {
		c.Fatalf("expected password `bar`, got %s", a.Password)
	}
	if a.IdentityToken != "" {
		c.Fatalf("expected identity token to be empty, got %s", a.IdentityToken)
	}
	if a.Email != "foo@example.com" {
		c.Fatalf("expected email `foo@example.com`, got %s", a.Email)
	}
}

func (s *DockerSuite) TestNativeStoreGetIdentityToken(c *check.C) {
	f := newConfigFile(map[string]types.AuthConfig{
		validServerAddress2: {
			Email: "foo@example2.com",
		},
	})
	f.CredentialsStore = "mock"

	ns := &nativeStore{
		programFunc: mockCommandFn,
		fileStore:   NewFileStore(f),
	}
	a, err := ns.Get(validServerAddress2)
	if err != nil {
		c.Fatal(err)
	}

	if a.Username != "" {
		c.Fatalf("expected username to be empty, got %s", a.Username)
	}
	if a.Password != "" {
		c.Fatalf("expected password to be empty, got %s", a.Password)
	}
	if a.IdentityToken != "abcd1234" {
		c.Fatalf("expected identity token `abcd1234`, got %s", a.IdentityToken)
	}
	if a.Email != "foo@example2.com" {
		c.Fatalf("expected email `foo@example2.com`, got %s", a.Email)
	}
}

func (s *DockerSuite) TestNativeStoreGetAll(c *check.C) {
	f := newConfigFile(map[string]types.AuthConfig{
		validServerAddress: {
			Email: "foo@example.com",
		},
		validServerAddress2: {
			Email: "foo@example2.com",
		},
	})
	f.CredentialsStore = "mock"

	ns := &nativeStore{
		programFunc: mockCommandFn,
		fileStore:   NewFileStore(f),
	}
	as, err := ns.GetAll()
	if err != nil {
		c.Fatal(err)
	}

	if len(as) != 2 {
		c.Fatalf("wanted 2, got %d", len(as))
	}

	if as[validServerAddress].Username != "foo" {
		c.Fatalf("expected username `foo` for %s, got %s", validServerAddress, as[validServerAddress].Username)
	}
	if as[validServerAddress].Password != "bar" {
		c.Fatalf("expected password `bar` for %s, got %s", validServerAddress, as[validServerAddress].Password)
	}
	if as[validServerAddress].IdentityToken != "" {
		c.Fatalf("expected identity to be empty for %s, got %s", validServerAddress, as[validServerAddress].IdentityToken)
	}
	if as[validServerAddress].Email != "foo@example.com" {
		c.Fatalf("expected email `foo@example.com` for %s, got %s", validServerAddress, as[validServerAddress].Email)
	}
	if as[validServerAddress2].Username != "" {
		c.Fatalf("expected username to be empty for %s, got %s", validServerAddress2, as[validServerAddress2].Username)
	}
	if as[validServerAddress2].Password != "" {
		c.Fatalf("expected password to be empty for %s, got %s", validServerAddress2, as[validServerAddress2].Password)
	}
	if as[validServerAddress2].IdentityToken != "abcd1234" {
		c.Fatalf("expected identity token `abcd1324` for %s, got %s", validServerAddress2, as[validServerAddress2].IdentityToken)
	}
	if as[validServerAddress2].Email != "foo@example2.com" {
		c.Fatalf("expected email `foo@example2.com` for %s, got %s", validServerAddress2, as[validServerAddress2].Email)
	}
}

func (s *DockerSuite) TestNativeStoreGetMissingCredentials(c *check.C) {
	f := newConfigFile(map[string]types.AuthConfig{
		validServerAddress: {
			Email: "foo@example.com",
		},
	})
	f.CredentialsStore = "mock"

	ns := &nativeStore{
		programFunc: mockCommandFn,
		fileStore:   NewFileStore(f),
	}
	_, err := ns.Get(missingCredsAddress)
	if err != nil {
		// missing credentials do not produce an error
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestNativeStoreGetInvalidAddress(c *check.C) {
	f := newConfigFile(map[string]types.AuthConfig{
		validServerAddress: {
			Email: "foo@example.com",
		},
	})
	f.CredentialsStore = "mock"

	ns := &nativeStore{
		programFunc: mockCommandFn,
		fileStore:   NewFileStore(f),
	}
	_, err := ns.Get(invalidServerAddress)
	if err == nil {
		c.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "program failed") {
		c.Fatalf("expected `program failed`, got %v", err)
	}
}

func (s *DockerSuite) TestNativeStoreErase(c *check.C) {
	f := newConfigFile(map[string]types.AuthConfig{
		validServerAddress: {
			Email: "foo@example.com",
		},
	})
	f.CredentialsStore = "mock"

	ns := &nativeStore{
		programFunc: mockCommandFn,
		fileStore:   NewFileStore(f),
	}
	err := ns.Erase(validServerAddress)
	if err != nil {
		c.Fatal(err)
	}

	if len(f.AuthConfigs) != 0 {
		c.Fatalf("expected 0 auth configs, got %d", len(f.AuthConfigs))
	}
}

func (s *DockerSuite) TestNativeStoreEraseInvalidAddress(c *check.C) {
	f := newConfigFile(map[string]types.AuthConfig{
		validServerAddress: {
			Email: "foo@example.com",
		},
	})
	f.CredentialsStore = "mock"

	ns := &nativeStore{
		programFunc: mockCommandFn,
		fileStore:   NewFileStore(f),
	}
	err := ns.Erase(invalidServerAddress)
	if err == nil {
		c.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "program failed") {
		c.Fatalf("expected `program failed`, got %v", err)
	}
}
