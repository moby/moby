package certutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/internal/test"
	"gotest.tools/assert"
	"gotest.tools/icmd"
)

// TLSConfig stores the tls certificate paths on disk
type TLSConfig struct {
	CACertPath string
	KeyPath    string
	CertPath   string
	certBase   string
}

// Cleanup removes all the certificates from the system
func (c TLSConfig) Cleanup(t assert.TestingT) {
	assert.Check(t, os.RemoveAll(c.certBase))
}

// New creates a new set of certificates useable for the provided SANs.
func New(t assert.TestingT, sans ...string) (TLSConfig, func(assert.TestingT)) {
	return newCerts(t, false, sans...)
}

// NewForClient creates a new set of certificates useable for the provided SANs.
func NewForClient(t assert.TestingT, sans ...string) (TLSConfig, func(assert.TestingT)) {
	return newCerts(t, true, sans...)
}

func newCerts(t assert.TestingT, client bool, sans ...string) (TLSConfig, func(assert.TestingT)) {
	if th, ok := t.(test.HelperT); ok {
		th.Helper()
	}

	n := t.(named)
	dir := filepath.Join(os.TempDir(), n.Name())
	err := os.MkdirAll(dir, 0700)
	assert.NilError(t, err, "error creating base store path")

	defer func() {
		if t.(failed).Failed() {
			os.RemoveAll(dir)
		}
	}()

	caRoot := filepath.Join(dir, "ca")
	certName := "cert.pem"
	keyName := "key.pem"
	if client {
		certName = "cert-client.pem"
		keyName = "key-client.pem"
	}

	certsRoot := filepath.Join(dir, "certs")
	assert.NilError(t, os.MkdirAll(certsRoot, 0700))
	cfg := TLSConfig{
		certBase:   dir,
		CACertPath: filepath.Join(caRoot, "rootCA.pem"),
		CertPath:   filepath.Join(certsRoot, certName),
		KeyPath:    filepath.Join(certsRoot, keyName),
	}

	cmd := icmd.Command("mkcert",
		append([]string{"--cert-file", cfg.CertPath, "--key-file", cfg.KeyPath, fmt.Sprintf("--client=%v", client)}, sans...)...,
	)
	cmd.Env = []string{"CAROOT=" + caRoot}
	icmd.RunCmd(cmd).Assert(t, icmd.Success)

	return cfg, cfg.Cleanup
}

type named interface {
	Name() string
}

type failed interface {
	Failed() bool
}
