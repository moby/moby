package registry

import (
	"encoding/pem"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/docker/libtrust"
)

func TestClientCertLoading(t *testing.T) {
	// There should already exist one custom certificate from our mock HTTPS
	// server. Ensure there it at least one subject in the CA Pool. There
	// should also be no client certs so far.
	certLock.RLock()
	if len(caPool.Subjects()) != 1 {
		defer certLock.RUnlock()
		t.Fatalf("expected 1 cert in the CA pool, got %d", len(caPool.Subjects()))
	}
	if len(clientCerts) != 0 {
		defer certLock.RUnlock()
		t.Fatalf("expected 0 certs in the client certs list, got %d", len(clientCerts))
	}
	certLock.RUnlock()

	// Now create a new ca cert and client key/cert for a bogus host.
	bogusCertsDirname := path.Join(certsDirname, "example.com")
	if err := os.MkdirAll(bogusCertsDirname, os.FileMode(0755)); err != nil {
		t.Fatalf("unable to create certs.d directory: %s", err)
	}

	key, err := libtrust.GenerateECP256PrivateKey()
	if err != nil {
		t.Fatalf("unable to generate key: %s", err)
	}

	// Generate CA certificate.
	caCert, err := libtrust.GenerateCACert(key, key.PublicKey())
	if err != nil {
		t.Fatalf("unable to generate self-signed CA cert: %s", err)
	}

	// Generate Client certificate.
	clientCert, err := libtrust.GenerateSelfSignedClientCert(key)
	if err != nil {
		t.Fatalf("unabel to generate self-signed client cert: %s", err)
	}

	// Write CA certificate file.
	caCertPEM := pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCert.Raw,
	}

	if err = ioutil.WriteFile(path.Join(bogusCertsDirname, "ca.crt"), pem.EncodeToMemory(&caCertPEM), os.FileMode(644)); err != nil {
		t.Fatalf("unable to write test ca cert file: %s", err)
	}

	// Write client certificate file.
	clientCertPEM := pem.Block{
		Type:  "CERTIFICATE",
		Bytes: clientCert.Raw,
	}

	if err = ioutil.WriteFile(path.Join(bogusCertsDirname, "client.cert"), pem.EncodeToMemory(&clientCertPEM), os.FileMode(644)); err != nil {
		t.Fatalf("unable to write test client cert file: %s", err)
	}

	// Write client key certificate file.
	clientKeyPEM, err := key.PEMBlock()
	if err != nil {
		t.Fatalf("unable to get clinet key PEM block: %s", err)
	}

	if err = ioutil.WriteFile(path.Join(bogusCertsDirname, "client.key"), pem.EncodeToMemory(clientKeyPEM), os.FileMode(644)); err != nil {
		t.Fatalf("unable to write test client cert file: %s", err)
	}

	// Now that we've written some new certs/keys, they should automatically
	// be reloaded into the global caPool and clientCerts objects. There is a
	// race condition here, so we wait for 100 milliseconds in case the watcher
	// needs the time to catch up and update the certs.
	time.Sleep(100 * time.Millisecond)

	// Now there should be 2 certs in the ca pool, and 1 client certificate.
	certLock.RLock()
	if len(caPool.Subjects()) != 2 {
		defer certLock.RUnlock()
		t.Fatalf("expected 2 certs in the CA pool, got %d", len(caPool.Subjects()))
	}
	if len(clientCerts) != 1 {
		defer certLock.RUnlock()
		t.Fatalf("expected 1 cert in the client certs list, got %d", len(clientCerts))
	}
	certLock.RUnlock()

	// Now remove the bogus server cert directory and wait for another update.
	if err = os.RemoveAll(bogusCertsDirname); err != nil {
		t.Fatalf("unable to remove test certs directory: %s", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Now there should be 1 certs in the ca pool again, and 0 client certificates.
	certLock.RLock()
	if len(caPool.Subjects()) != 1 {
		defer certLock.RUnlock()
		t.Fatalf("expected 1 cert in the CA pool, got %d", len(caPool.Subjects()))
	}
	if len(clientCerts) != 0 {
		defer certLock.RUnlock()
		t.Fatalf("expected 0 certs in the client certs list, got %d", len(clientCerts))
	}
	certLock.RUnlock()
}
