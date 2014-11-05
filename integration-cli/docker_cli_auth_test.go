package main

import (
	"os/exec"
	"strings"
	"testing"
)

const testDaemonURL = "tcp://localhost:4272"

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
