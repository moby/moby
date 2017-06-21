package manifest

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/docker/image"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
)

func TestStoreGetSet(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "manifest-store")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	store, err := NewManifestStore(tmpdir)
	require.NoError(t, err)

	type tcase struct {
		input []byte
		ref   string
	}
	tcases := []tcase{
		{[]byte(`{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","layers":[],"config":{}}`),
			"sha256:d5f0a331a44afef0062260bb42d0488b0e3bf9d3639ae963369cfd6a2908c483"},
	}

	for _, tc := range tcases {
		id, err := store.Set("img", manifestFromBytes(tc.input))
		require.NoError(t, err)
		require.Equal(t, tc.ref, id.String(), "Expected ID %q, got %q.", tc.ref, id)
	}

	for _, tc := range tcases {
		ref, err := newReference("img@" + tc.ref)
		require.NoError(t, err)
		m, err := store.Get(ref)
		require.NoError(t, err)
		_, data, err := (*m).Payload()
		require.NoError(t, err)

		if !bytes.Equal(data, tc.input) {
			t.Fatalf("Expected data %q, got %q", tc.input, data)
		}
	}

	for _, badDigest := range []string{"img@foobar:abc", "img@sha256:abc", "img@sha256:d5f0a331a44afef0062260bb42d0488b0e3bf9d3639ae963369cfd6a2908c483a"} {
		ref, err := newReference(badDigest)
		require.Error(t, err, "Expected error for ID %q.", ref)
	}
}

func TestStoreGetInvalidData(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "manifest-store")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	store, err := NewManifestStore(tmpdir)
	require.NoError(t, err)

	mBytes := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","layers":[],"config":{}}`)
	id, err := store.Set("img", manifestFromBytes(mBytes))
	require.NoError(t, err)

	ref, err := newReference(fmt.Sprintf("img@%s", id))
	require.NoError(t, err)

	mBytes2 := []byte(`{"schemaVersion":2,   "mediaType":"application/vnd.docker.distribution.manifest.v2+json","layers":[],"config":{}}`)
	base46Ref := base64.StdEncoding.EncodeToString([]byte(ref.String()))
	if err := ioutil.WriteFile(filepath.Join(tmpdir, contentDirName, string(digest.Canonical), base46Ref), mBytes2, 0600); err != nil {
		t.Fatal(err)
	}

	_, err = store.Get(ref)
	require.Error(t, err, "Expected get to fail after data modification.")
}

func TestStoreInvalidRoot(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "manifest-store")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	tcases := []struct {
		root, invalidFile string
	}{
		{"root", "root"},
		{"root", "root/content"},
	}

	for _, tc := range tcases {
		root := filepath.Join(tmpdir, tc.root)
		filePath := filepath.Join(tmpdir, tc.invalidFile)
		err := os.MkdirAll(filepath.Dir(filePath), 0700)
		require.NoError(t, err)
		f, err := os.Create(filePath)
		require.NoError(t, err)
		f.Close()

		_, err = NewManifestStore(root)
		require.Error(t, err, "Expected error from root %q and invalid file %q.", tc.root, tc.invalidFile)

		os.RemoveAll(root)
	}
}

func TestStoreDelete(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "manifest-store")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	store, err := NewManifestStore(tmpdir)
	require.NoError(t, err)

	mBytes := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","layers":[],"config":{}}`)
	id, err := store.Set("img", manifestFromBytes(mBytes))
	require.NoError(t, err)

	ref, _ := newReference(fmt.Sprintf("img@%s", id))
	_, err = store.Get(ref)
	require.NoError(t, err)

	err = store.Delete(ref)
	require.NoError(t, err)

	_, err = store.Get(ref)
	require.Error(t, err, "Expected error for retrieving an image after pruning.")
}

func TestStoreGetDeleteByImageID(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "manifest-store")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	store, err := NewManifestStore(tmpdir)
	require.NoError(t, err)

	mBytes := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","layers":[],
	"config":{"digest":"sha256:d5f0a331a44afef0062260bb42d0488b0e3bf9d3639ae963369cfd6a2908c483"}}`)
	expectedImageID := image.ID("sha256:d5f0a331a44afef0062260bb42d0488b0e3bf9d3639ae963369cfd6a2908c483")
	id, err := store.Set("img", manifestFromBytes(mBytes))
	require.NoError(t, err)

	ref, _ := newReference(fmt.Sprintf("img@%s", id))
	imageID, err := store.GetImageID(ref)
	require.NoError(t, err)
	require.Equal(t, expectedImageID, imageID)

	err = store.DeleteByImageID(imageID)
	require.NoError(t, err)

	_, err = store.Get(ref)
	require.Error(t, err, "Expected error for retrieving an image after pruning.")
}

func manifestFromBytes(data []byte) schema2.DeserializedManifest {
	m := new(schema2.DeserializedManifest)
	m.UnmarshalJSON(data)
	return *m
}
