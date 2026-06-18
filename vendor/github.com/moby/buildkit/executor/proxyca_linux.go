//go:build linux

package executor

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"syscall"

	"github.com/containerd/continuity/fs"
	"github.com/pkg/errors"
)

var linuxSystemCertFiles = []string{
	"/etc/ssl/certs/ca-certificates.crt",
	"/etc/pki/tls/certs/ca-bundle.crt",
	"/etc/ssl/ca-bundle.pem",
	"/etc/pki/tls/cacert.pem",
	"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem",
	"/etc/ssl/cert.pem",
}

var (
	proxyCABegin = []byte("\n# buildkit proxy CA begin\n")
	proxyCAEnd   = []byte("# buildkit proxy CA end\n")
)

// InjectProxyCA appends caPEM to the rootfs trust bundle used by common Linux
// TLS stacks and returns a cleanup that removes only the injected CA.
func InjectProxyCA(rootfsPath string, caPEM []byte) (func() error, error) {
	if len(caPEM) == 0 {
		return func() error { return nil }, nil
	}
	cert, err := firstCertificate(caPEM)
	if err != nil {
		return nil, err
	}
	certSum := sha256.Sum256(cert.Raw)

	var bundle string
	for _, name := range linuxSystemCertFiles {
		p, err := fs.RootPath(rootfsPath, name)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve certificate bundle %s", name)
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			bundle = p
			break
		}
	}
	if bundle == "" {
		return func() error { return nil }, nil
	}

	original, err := os.ReadFile(bundle)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if containsCertificate(original, certSum) {
		return func() error { return nil }, nil
	}
	st, err := os.Stat(bundle)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	next := append([]byte{}, original...)
	if len(next) > 0 && next[len(next)-1] != '\n' {
		next = append(next, '\n')
	}
	next = append(next, proxyCABegin...)
	next = append(next, caPEM...)
	if len(next) > 0 && next[len(next)-1] != '\n' {
		next = append(next, '\n')
	}
	next = append(next, proxyCAEnd...)
	if err := writeCertBundle(bundle, next, st); err != nil {
		return nil, err
	}

	return func() error {
		current, err := os.ReadFile(bundle)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return errors.WithStack(err)
		}
		cleaned := removeInjectedCA(current, certSum)
		if bytes.Equal(current, cleaned) {
			return nil
		}
		st, err := os.Stat(bundle)
		if err != nil {
			return errors.WithStack(err)
		}
		return writeCertBundle(bundle, cleaned, st)
	}, nil
}

func firstCertificate(dt []byte) (*x509.Certificate, error) {
	for {
		block, rest := pem.Decode(dt)
		if block == nil {
			return nil, errors.New("proxy CA PEM does not contain a certificate")
		}
		dt = rest
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return cert, nil
	}
}

func containsCertificate(dt []byte, sum [sha256.Size]byte) bool {
	for {
		block, rest := pem.Decode(dt)
		if block == nil {
			return false
		}
		dt = rest
		if block.Type != "CERTIFICATE" {
			continue
		}
		if sha256.Sum256(block.Bytes) == sum {
			return true
		}
	}
}

func removeInjectedCA(dt []byte, sum [sha256.Size]byte) []byte {
	begin := bytes.Index(dt, proxyCABegin)
	if begin >= 0 {
		end := bytes.Index(dt[begin+len(proxyCABegin):], proxyCAEnd)
		if end >= 0 {
			end += begin + len(proxyCABegin) + len(proxyCAEnd)
			block := dt[begin:end]
			if containsCertificate(block, sum) {
				out := append([]byte{}, dt[:begin]...)
				out = append(out, dt[end:]...)
				return out
			}
		}
	}

	var out []byte
	for len(dt) > 0 {
		idx := bytes.Index(dt, []byte("-----BEGIN "))
		if idx < 0 {
			out = append(out, dt...)
			break
		}
		out = append(out, dt[:idx]...)
		block, rest := pem.Decode(dt[idx:])
		if block == nil {
			out = append(out, dt[idx:]...)
			break
		}
		consumed := len(dt[idx:]) - len(rest)
		if block.Type != "CERTIFICATE" || sha256.Sum256(block.Bytes) != sum {
			out = append(out, dt[idx:idx+consumed]...)
		}
		dt = rest
	}
	return out
}

func writeCertBundle(path string, dt []byte, st os.FileInfo) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".buildkit-ca-*")
	if err != nil {
		return errors.WithStack(err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(dt); err != nil {
		tmp.Close()
		return errors.WithStack(err)
	}
	if err := tmp.Chmod(st.Mode()); err != nil {
		tmp.Close()
		return errors.WithStack(err)
	}
	if sys, ok := st.Sys().(*syscall.Stat_t); ok {
		if err := tmp.Chown(int(sys.Uid), int(sys.Gid)); err != nil {
			tmp.Close()
			return errors.WithStack(err)
		}
	}
	if err := tmp.Close(); err != nil {
		return errors.WithStack(err)
	}
	return errors.WithStack(os.Rename(tmpName, path))
}
