package main

import (
	"os/exec"
	"strings"
	"testing"
)

const testDaemonURL = "tcp://localhost:4272"

func TestAuthIdentityWithGoodKeys(t *testing.T) {
	d := NewDaemon(t)
	if err := d.Start(
		"-H", testDaemonURL,
		"--auth", "identity",
		"--identity", "fixtures/https/private-key-1.json",
		"--auth-authorized-keys", "fixtures/https/public-key-2.json",
	); err != nil {
		t.Fatalf("Could not start daemon: %v", err)
	}
	defer d.Stop()

	// ensure basic connection
	cmd := exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--auth", "identity",
		"--identity", "fixtures/https/private-key-2.json",
		"--auth-known-hosts", "fixtures/https/public-key-1.json",
		"info",
	)
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatalf("failed to execute command: %s, %v", out, err)
	}

	logDone("auth - identity mode with good keys")
}

func TestAuthIdentityWithBadServerKey(t *testing.T) {
	d := NewDaemon(t)
	if err := d.Start(
		"-H", testDaemonURL,
		"--auth", "identity",
		"--identity", "fixtures/https/private-key-3.json",
		"--auth-authorized-keys", "fixtures/https/public-key-2.json",
	); err != nil {
		t.Fatalf("Could not start daemon: %v", err)
	}
	defer d.Stop()

	// ensure basic connection
	cmd := exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--auth", "identity",
		"--identity", "fixtures/https/private-key-2.json",
		"--auth-known-hosts", "fixtures/https/public-key-1.json",
		"info",
	)
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatalf("command should have failed: %s", out)
	}
	if !strings.Contains(out, "The authenticity of host \"localhost:4272\" can't be established.") {
		t.Errorf("command output should have contained 'The authenticity of host \"localhost:4272\" can't be established.': %s", out)
	}

	logDone("auth - identity mode with bad server key")
}

func TestAuthIdentityWithBadClientKey(t *testing.T) {
	d := NewDaemon(t)
	if err := d.Start(
		"-H", testDaemonURL,
		"--auth", "identity",
		"--identity", "fixtures/https/private-key-1.json",
		"--auth-authorized-keys", "fixtures/https/public-key-2.json",
	); err != nil {
		t.Fatalf("Could not start daemon: %v", err)
	}
	defer d.Stop()

	// ensure basic connection
	cmd := exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--auth", "identity",
		"--identity", "fixtures/https/private-key-3.json",
		"--auth-known-hosts", "fixtures/https/public-key-1.json",
		"info",
	)
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatalf("command should have failed: %s", out)
	}
	if !strings.Contains(out, "remote error: bad certificate") {
		t.Errorf("command output should have contained 'remote error: bad certificate': %s", out)
	}

	logDone("auth - identity mode with bad client key")
}

func TestAuthCert(t *testing.T) {
	d := NewDaemon(t)
	if err := d.Start(
		"-H", testDaemonURL,
		"--auth", "cert",
		"--auth-cert", "fixtures/https/server-cert.pem",
		"--auth-key", "fixtures/https/server-key.pem",
	); err != nil {
		t.Fatalf("Could not start daemon: %v", err)
	}
	defer d.Stop()

	// ensure basic TLS connection
	cmd := exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--auth", "cert",
		"info",
	)
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatalf("failed to execute command: %s, %v", out, err)
	}

	// ensure verification with CA works
	cmd = exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--auth", "cert",
		"--auth-ca", "fixtures/https/ca.pem",
		"info",
	)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatalf("failed to execute command: %s, %v", out, err)
	}

	// ensure connecting to server without TLS enabled fails
	cmd = exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--auth", "none",
		"info",
	)
	out, _, err = runCommandWithOutput(cmd)
	if err == nil {
		t.Fatalf("command with --auth=none should have failed: %s", out)
	}
	if !strings.Contains(out, "malformed HTTP response") {
		t.Errorf("command output should have contained 'malformed HTTP response': %s", out)
	}

	logDone("auth - cert mode, client verifying server")
}

func TestAuthCertWithBadCert(t *testing.T) {
	d := NewDaemon(t)
	if err := d.Start(
		"-H", testDaemonURL,
		"--auth", "cert",
		"--auth-cert", "fixtures/https/server-rogue-cert.pem",
		"--auth-key", "fixtures/https/server-rogue-key.pem",
	); err != nil {
		t.Fatalf("Could not start daemon: %v", err)
	}
	defer d.Stop()

	// ensure verification fails
	cmd := exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--auth", "cert",
		"--auth-ca", "fixtures/https/ca.pem",
		"info",
	)
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatalf("command without cert and key should have failed: %s", out)
	}
	if !strings.Contains(out, "certificate signed by unknown authority") {
		t.Errorf("command output should have contained 'certificate signed by unknown authority': %s", out)
	}

	logDone("auth - cert mode, client rejects bad certificate")
}

func TestAuthCertClientCert(t *testing.T) {
	d := NewDaemon(t)
	if err := d.Start(
		"-H", testDaemonURL,
		"--auth", "cert",
		"--auth-cert", "fixtures/https/server-cert.pem",
		"--auth-key", "fixtures/https/server-key.pem",
		"--auth-ca", "fixtures/https/ca.pem",
	); err != nil {
		t.Fatalf("Could not start daemon: %v", err)
	}
	defer d.Stop()

	// ensure basic client cert works
	cmd := exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--auth", "cert",
		"--auth-cert", "fixtures/https/client-cert.pem",
		"--auth-key", "fixtures/https/client-key.pem",
		"info",
	)
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatalf("failed to execute command: %s, %v", out, err)
	}

	// ensure client cert with --tlsverify works
	cmd = exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--auth", "cert",
		"--auth-cert", "fixtures/https/client-cert.pem",
		"--auth-key", "fixtures/https/client-key.pem",
		"--auth-ca", "fixtures/https/ca.pem",
		"info",
	)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatalf("failed to execute command: %s, %v", out, err)
	}

	// ensure not passing certs fails
	cmd = exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--auth", "cert",
		"info",
	)
	out, _, err = runCommandWithOutput(cmd)
	if err == nil {
		t.Fatalf("command should have failed: %s", out)
	}
	if !strings.Contains(out, "bad certificate") {
		t.Errorf("command output should have contained 'bad certificate': %s", out)
	}

	// ensure passing incorrect certs fails
	cmd = exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--auth", "cert",
		"--auth-cert", "fixtures/https/client-rogue-cert.pem",
		"--auth-key", "fixtures/https/client-rogue-key.pem",
		"info",
	)
	out, _, err = runCommandWithOutput(cmd)
	if err == nil {
		t.Fatalf("command should have failed: %s", out)
	}
	if !strings.Contains(out, "bad certificate") {
		t.Errorf("command output should have contained 'bad certificate': %s", out)
	}

	logDone("auth - cert mode, client certificates")
}

func TestAuthTLS(t *testing.T) {
	d := NewDaemon(t)
	if err := d.Start(
		"-H", testDaemonURL,
		"--tls",
		"--tlscert", "fixtures/https/server-cert.pem",
		"--tlskey", "fixtures/https/server-key.pem",
	); err != nil {
		t.Fatalf("Could not start daemon: %v", err)
	}
	defer d.Stop()

	// ensure basic TLS connection
	cmd := exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--tls",
		"info",
	)
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatalf("failed to execute command: %s, %v", out, err)
	}

	// ensure basic TLS connection is triggered with --tlsverify=false
	cmd = exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--tlsverify=false",
		"info",
	)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatalf("failed to execute command: %s, %v", out, err)
	}

	// ensure verification with CA works
	cmd = exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--tlsverify",
		"--tlscacert", "fixtures/https/ca.pem",
		"info",
	)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatalf("failed to execute command: %s, %v", out, err)
	}

	// ensure connecting to server without TLS enabled fails
	cmd = exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--auth", "none",
		"info",
	)
	out, _, err = runCommandWithOutput(cmd)
	if err == nil {
		t.Fatalf("command without --tls should have failed: %s", out)
	}
	if !strings.Contains(out, "malformed HTTP response") {
		t.Errorf("command output should have contained 'malformed HTTP response': %s", out)
	}

	logDone("auth - client verifying server with TLS options")
}

func TestAuthTLSVerifyWithBadCert(t *testing.T) {
	d := NewDaemon(t)
	if err := d.Start(
		"-H", testDaemonURL,
		"--tls",
		"--tlscert", "fixtures/https/server-rogue-cert.pem",
		"--tlskey", "fixtures/https/server-rogue-key.pem",
	); err != nil {
		t.Fatalf("Could not start daemon: %v", err)
	}
	defer d.Stop()

	// ensure verification fails
	cmd := exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--tlsverify",
		"--tlscacert", "fixtures/https/ca.pem",
		"info",
	)
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatalf("command without cert and key should have failed: %s", out)
	}
	if !strings.Contains(out, "certificate signed by unknown authority") {
		t.Errorf("command output should have contained 'certificate signed by unknown authority': %s", out)
	}

	logDone("auth - client rejects bad certificate with TLS options")
}

func TestAuthTLSClientCert(t *testing.T) {
	d := NewDaemon(t)
	if err := d.Start(
		"-H", testDaemonURL,
		"--tlsverify",
		"--tlscert", "fixtures/https/server-cert.pem",
		"--tlskey", "fixtures/https/server-key.pem",
		"--tlscacert", "fixtures/https/ca.pem",
	); err != nil {
		t.Fatalf("Could not start daemon: %v", err)
	}
	defer d.Stop()

	// ensure basic client cert works
	cmd := exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--tls",
		"--tlscert", "fixtures/https/client-cert.pem",
		"--tlskey", "fixtures/https/client-key.pem",
		"info",
	)
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatalf("failed to execute command: %s, %v", out, err)
	}

	// ensure client cert with --tlsverify works
	cmd = exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--tlsverify",
		"--tlscert", "fixtures/https/client-cert.pem",
		"--tlskey", "fixtures/https/client-key.pem",
		"--tlscacert", "fixtures/https/ca.pem",
		"info",
	)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatalf("failed to execute command: %s, %v", out, err)
	}

	// ensure not passing certs fails
	cmd = exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--tls",
		"info",
	)
	out, _, err = runCommandWithOutput(cmd)
	if err == nil {
		t.Fatalf("command should have failed: %s", out)
	}
	if !strings.Contains(out, "bad certificate") {
		t.Errorf("command output should have contained 'bad certificate': %s", out)
	}

	// ensure passing incorrect certs fails
	cmd = exec.Command(
		dockerBinary,
		"-H", testDaemonURL,
		"--tls",
		"--tlscert", "fixtures/https/client-rogue-cert.pem",
		"--tlskey", "fixtures/https/client-rogue-key.pem",
		"info",
	)
	out, _, err = runCommandWithOutput(cmd)
	if err == nil {
		t.Fatalf("command should have failed: %s", out)
	}
	if !strings.Contains(out, "bad certificate") {
		t.Errorf("command output should have contained 'bad certificate': %s", out)
	}

	logDone("auth - client certificates with TLS options")
}
