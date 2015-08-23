package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/docker/distribution/digest"
	"github.com/go-check/check"
)

const v2binary = "registry-v2"

type testRegistryV2 struct {
	cmd *exec.Cmd
	dir string
}

func newTestRegistryV2(c *check.C) (*testRegistryV2, error) {
	template := `version: 0.1
loglevel: debug
storage:
    filesystem:
        rootdirectory: %s
http:
    addr: %s`
	tmp, err := ioutil.TempDir("", "registry-test-")
	if err != nil {
		return nil, err
	}
	confPath := filepath.Join(tmp, "config.yaml")
	config, err := os.Create(confPath)
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(config, template, tmp, privateRegistryURL); err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}

	cmd := exec.Command(v2binary, confPath)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(tmp)
		if os.IsNotExist(err) {
			c.Skip(err.Error())
		}
		return nil, err
	}
	return &testRegistryV2{
		cmd: cmd,
		dir: tmp,
	}, nil
}

func (t *testRegistryV2) Ping() error {
	// We always ping through HTTP for our test registry.
	resp, err := http.Get(fmt.Sprintf("http://%s/v2/", privateRegistryURL))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("registry ping replied with an unexpected status code %d", resp.StatusCode)
	}
	return nil
}

func (t *testRegistryV2) Close() {
	t.cmd.Process.Kill()
	os.RemoveAll(t.dir)
}

func (t *testRegistryV2) getBlobFilename(blobDigest digest.Digest) string {
	// Split the digest into it's algorithm and hex components.
	dgstAlg, dgstHex := blobDigest.Algorithm(), blobDigest.Hex()

	// The path to the target blob data looks something like:
	//   baseDir + "docker/registry/v2/blobs/sha256/a3/a3ed...46d4/data"
	return fmt.Sprintf("%s/docker/registry/v2/blobs/%s/%s/%s/data", t.dir, dgstAlg, dgstHex[:2], dgstHex)
}

func (t *testRegistryV2) readBlobContents(c *check.C, blobDigest digest.Digest) []byte {
	// Load the target manifest blob.
	manifestBlob, err := ioutil.ReadFile(t.getBlobFilename(blobDigest))
	if err != nil {
		c.Fatalf("unable to read blob: %s", err)
	}

	return manifestBlob
}

func (t *testRegistryV2) writeBlobContents(c *check.C, blobDigest digest.Digest, data []byte) {
	if err := ioutil.WriteFile(t.getBlobFilename(blobDigest), data, os.FileMode(0644)); err != nil {
		c.Fatalf("unable to write malicious data blob: %s", err)
	}
}

func (t *testRegistryV2) tempMoveBlobData(c *check.C, blobDigest digest.Digest) (undo func()) {
	tempFile, err := ioutil.TempFile("", "registry-temp-blob-")
	if err != nil {
		c.Fatalf("unable to get temporary blob file: %s", err)
	}
	tempFile.Close()

	blobFilename := t.getBlobFilename(blobDigest)

	// Move the existing data file aside, so that we can replace it with a
	// another blob of data.
	if err := os.Rename(blobFilename, tempFile.Name()); err != nil {
		os.Remove(tempFile.Name())
		c.Fatalf("unable to move data blob: %s", err)
	}

	return func() {
		os.Rename(tempFile.Name(), blobFilename)
		os.Remove(tempFile.Name())
	}
}
