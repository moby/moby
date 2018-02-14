package registry // import "github.com/docker/docker/integration-cli/registry"

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/go-connections/sockets"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

const (
	v2binary        = "registry-v2"
	v2binarySchema1 = "registry-v2-schema1"
)

type testingT interface {
	logT
	Fatal(...interface{})
	Fatalf(string, ...interface{})
}

type logT interface {
	Logf(string, ...interface{})
}

// V2 represent a registry version 2
type V2 struct {
	cmd         *exec.Cmd
	registryURL string
	dir         string
	auth        string
	username    string
	password    string
	email       string
}

type V2Config struct {
	RegistryURL    string
	Schema1        bool
	Auth           string
	TokenURL       string
	ProxyRemoteURL string // requires scheme part (http:// or unix://)
}

// NewV2 creates a v2 registry server
func NewV2(c V2Config) (*V2, error) {
	if strings.Count(c.RegistryURL, "://") != 1 {
		return nil, errors.Errorf("unexpected registry URL: %q", c.RegistryURL)
	}
	tmp, err := ioutil.TempDir("", "registry-test-")
	if err != nil {
		return nil, err
	}
	net := "tcp"
	addr := strings.TrimPrefix(c.RegistryURL, "http://")
	if strings.HasPrefix(c.RegistryURL, "unix://") {
		net = "unix"
		addr = strings.TrimPrefix(c.RegistryURL, "unix://")
	}
	template := fmt.Sprintf(`version: 0.1
loglevel: debug
storage:
    cache:
        blobdescriptor: inmemory
    filesystem:
        rootdirectory: %s
http:
    addr: %s
    net: %s
`, tmp, addr, net)
	var (
		authTemplate  string
		proxyTemplate string
		username      string
		password      string
		email         string
	)
	switch c.Auth {
	case "htpasswd":
		htpasswdPath := filepath.Join(tmp, "htpasswd")
		// generated with: htpasswd -Bbn testuser testpassword
		userpasswd := "testuser:$2y$05$sBsSqk0OpSD1uTZkHXc4FeJ0Z70wLQdAX/82UiHuQOKbNbBrzs63m"
		username = "testuser"
		password = "testpassword"
		email = "test@test.org"
		if err := ioutil.WriteFile(htpasswdPath, []byte(userpasswd), os.FileMode(0644)); err != nil {
			return nil, err
		}
		authTemplate = fmt.Sprintf(`auth:
    htpasswd:
        realm: basic-realm
        path: %s
`, htpasswdPath)
	case "token":
		authTemplate = fmt.Sprintf(`auth:
    token:
        realm: %s
        service: "registry"
        issuer: "auth-registry"
        rootcertbundle: "fixtures/registry/cert.pem"
`, c.TokenURL)
	}
	if c.ProxyRemoteURL != "" {
		proxyTemplate = fmt.Sprintf(`proxy:
    remoteurl: %s
`, c.ProxyRemoteURL)
	}

	confPath := filepath.Join(tmp, "config.yaml")
	config, err := os.Create(confPath)
	if err != nil {
		return nil, err
	}
	defer config.Close()

	if _, err := fmt.Fprintf(config, strings.Join([]string{template, authTemplate, proxyTemplate}, "\n")); err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}

	binary := v2binary
	if c.Schema1 {
		binary = v2binarySchema1
	}
	cmd := exec.Command(binary, confPath)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}
	return &V2{
		cmd:         cmd,
		dir:         tmp,
		auth:        c.Auth,
		username:    username,
		password:    password,
		email:       email,
		registryURL: c.RegistryURL,
	}, nil
}

// Ping sends an http request to the current registry, and fail if it doesn't respond correctly
func (r *V2) Ping() error {
	reqURL := r.registryURL + "/v2/"
	var tr http.Transport
	if strings.HasPrefix(r.registryURL, "unix://") {
		reqURL = "http://registry/v2/"
		path := strings.TrimPrefix(r.registryURL, "unix://")
		if err := sockets.ConfigureTransport(&tr, "unix", path); err != nil {
			return err
		}
	}
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return err
	}
	client := http.Client{Transport: &tr}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	fail := resp.StatusCode != http.StatusOK
	if r.auth != "" {
		// unauthorized is a _good_ status when pinging v2/ and it needs auth
		fail = fail && resp.StatusCode != http.StatusUnauthorized
	}
	if fail {
		return fmt.Errorf("registry ping replied with an unexpected status code %d", resp.StatusCode)
	}
	return nil
}

// Close kills the registry server
func (r *V2) Close() {
	r.cmd.Process.Kill()
	r.cmd.Process.Wait()
	os.RemoveAll(r.dir)
}

func (r *V2) getBlobFilename(blobDigest digest.Digest) string {
	// Split the digest into its algorithm and hex components.
	dgstAlg, dgstHex := blobDigest.Algorithm(), blobDigest.Hex()

	// The path to the target blob data looks something like:
	//   baseDir + "docker/registry/v2/blobs/sha256/a3/a3ed...46d4/data"
	return fmt.Sprintf("%s/docker/registry/v2/blobs/%s/%s/%s/data", r.dir, dgstAlg, dgstHex[:2], dgstHex)
}

// ReadBlobContents read the file corresponding to the specified digest
func (r *V2) ReadBlobContents(t testingT, blobDigest digest.Digest) []byte {
	// Load the target manifest blob.
	manifestBlob, err := ioutil.ReadFile(r.getBlobFilename(blobDigest))
	if err != nil {
		t.Fatalf("unable to read blob: %s", err)
	}

	return manifestBlob
}

// WriteBlobContents write the file corresponding to the specified digest with the given content
func (r *V2) WriteBlobContents(t testingT, blobDigest digest.Digest, data []byte) {
	if err := ioutil.WriteFile(r.getBlobFilename(blobDigest), data, os.FileMode(0644)); err != nil {
		t.Fatalf("unable to write malicious data blob: %s", err)
	}
}

// TempMoveBlobData moves the existing data file aside, so that we can replace it with a
// malicious blob of data for example.
func (r *V2) TempMoveBlobData(t testingT, blobDigest digest.Digest) (undo func()) {
	tempFile, err := ioutil.TempFile("", "registry-temp-blob-")
	if err != nil {
		t.Fatalf("unable to get temporary blob file: %s", err)
	}
	tempFile.Close()

	blobFilename := r.getBlobFilename(blobDigest)

	// Move the existing data file aside, so that we can replace it with a
	// another blob of data.
	if err := os.Rename(blobFilename, tempFile.Name()); err != nil {
		os.Remove(tempFile.Name())
		t.Fatalf("unable to move data blob: %s", err)
	}

	return func() {
		os.Rename(tempFile.Name(), blobFilename)
		os.Remove(tempFile.Name())
	}
}

// ManifestDigest does not verify args, as this is just for test utility.
// usage: ManifestDigest("library/hello-world", latest)
func (r *V2) ManifestDigest(repo, tag string) (digest.Digest, error) {
	path := filepath.Join(r.Path(), "repositories", repo, "_manifests", "tags", tag, "current", "link")
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return digest.Digest(string(b)), nil
}

// Username returns the configured user name of the server
func (r *V2) Username() string {
	return r.username
}

// Password returns the configured password of the server
func (r *V2) Password() string {
	return r.password
}

// Email returns the configured email of the server
func (r *V2) Email() string {
	return r.email
}

// Path returns the path where the registry write data
func (r *V2) Path() string {
	return filepath.Join(r.dir, "docker", "registry", "v2")
}
