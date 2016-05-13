package layer

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/docker/docker/daemon/storage"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/stringid"
	"github.com/vbatts/tar-split/tar/asm"
	tarStorage "github.com/vbatts/tar-split/tar/storage"
)

func writeTarSplitFile(name string, tarContent []byte) error {
	f, err := os.OpenFile(name, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	fz := gzip.NewWriter(f)

	metaPacker := tarStorage.NewJSONPacker(fz)
	defer fz.Close()

	rdr, err := asm.NewInputTarStream(bytes.NewReader(tarContent), metaPacker, nil)
	if err != nil {
		return err
	}

	if _, err := io.Copy(ioutil.Discard, rdr); err != nil {
		return err
	}

	return nil
}

func TestLayerMigration(t *testing.T) {
	// TODO Windows: Figure out why this is failing
	if runtime.GOOS == "windows" {
		t.Skip("Failing on Windows")
	}
	td, err := ioutil.TempDir("", "migration-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(td)

	layer1Files := []FileApplier{
		newTestFile("/root/.bashrc", []byte("# Boring configuration"), 0644),
		newTestFile("/etc/profile", []byte("# Base configuration"), 0644),
	}

	layer2Files := []FileApplier{
		newTestFile("/root/.bashrc", []byte("# Updated configuration"), 0644),
	}

	tar1, err := tarFromFiles(layer1Files...)
	if err != nil {
		t.Fatal(err)
	}

	tar2, err := tarFromFiles(layer2Files...)
	if err != nil {
		t.Fatal(err)
	}

	storage, err := newVFSStorage(filepath.Join(td, "storage-"))
	if err != nil {
		t.Fatal(err)
	}

	storageID1 := stringid.GenerateRandomID()
	if err := storage.Create(storageID1, "", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.ApplyDiff(storageID1, "", archive.Reader(bytes.NewReader(tar1))); err != nil {
		t.Fatal(err)
	}

	tf1 := filepath.Join(td, "tar1.json.gz")
	if err := writeTarSplitFile(tf1, tar1); err != nil {
		t.Fatal(err)
	}

	fms, err := NewFSMetadataStore(filepath.Join(td, "layers"))
	if err != nil {
		t.Fatal(err)
	}
	ls, err := NewStoreFromStorage(fms, storage)
	if err != nil {
		t.Fatal(err)
	}

	newTarDataPath := filepath.Join(td, ".migration-tardata")
	diffID, size, err := ls.(*layerStore).ChecksumForStorageID(storageID1, "", tf1, newTarDataPath)
	if err != nil {
		t.Fatal(err)
	}

	layer1a, err := ls.(*layerStore).RegisterByStorageID(storageID1, "", diffID, newTarDataPath, size)
	if err != nil {
		t.Fatal(err)
	}

	layer1b, err := ls.Register(bytes.NewReader(tar1), "")
	if err != nil {
		t.Fatal(err)
	}

	assertReferences(t, layer1a, layer1b)
	// Attempt register, should be same
	layer2a, err := ls.Register(bytes.NewReader(tar2), layer1a.ChainID())
	if err != nil {
		t.Fatal(err)
	}

	storageID2 := stringid.GenerateRandomID()
	if err := storage.Create(storageID2, storageID1, "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.ApplyDiff(storageID2, storageID1, archive.Reader(bytes.NewReader(tar2))); err != nil {
		t.Fatal(err)
	}

	tf2 := filepath.Join(td, "tar2.json.gz")
	if err := writeTarSplitFile(tf2, tar2); err != nil {
		t.Fatal(err)
	}
	diffID, size, err = ls.(*layerStore).ChecksumForStorageID(storageID2, storageID1, tf2, newTarDataPath)
	if err != nil {
		t.Fatal(err)
	}

	layer2b, err := ls.(*layerStore).RegisterByStorageID(storageID2, layer1a.ChainID(), diffID, tf2, size)
	if err != nil {
		t.Fatal(err)
	}
	assertReferences(t, layer2a, layer2b)

	if metadata, err := ls.Release(layer2a); err != nil {
		t.Fatal(err)
	} else if len(metadata) > 0 {
		t.Fatalf("Unexpected layer removal after first release: %#v", metadata)
	}

	metadata, err := ls.Release(layer2b)
	if err != nil {
		t.Fatal(err)
	}

	assertMetadata(t, metadata, createMetadata(layer2a))
}

func tarFromFilesInStorage(storage storage.Driver, storageID, parentID string, files ...FileApplier) ([]byte, error) {
	t, err := tarFromFiles(files...)
	if err != nil {
		return nil, err
	}

	if err := storage.Create(storageID, parentID, "", nil); err != nil {
		return nil, err
	}
	if _, err := storage.ApplyDiff(storageID, parentID, archive.Reader(bytes.NewReader(t))); err != nil {
		return nil, err
	}

	ar, err := storage.Diff(storageID, parentID)
	if err != nil {
		return nil, err
	}
	defer ar.Close()

	return ioutil.ReadAll(ar)
}

func TestLayerMigrationNoTarsplit(t *testing.T) {
	// TODO Windows: Figure out why this is failing
	if runtime.GOOS == "windows" {
		t.Skip("Failing on Windows")
	}
	td, err := ioutil.TempDir("", "migration-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(td)

	layer1Files := []FileApplier{
		newTestFile("/root/.bashrc", []byte("# Boring configuration"), 0644),
		newTestFile("/etc/profile", []byte("# Base configuration"), 0644),
	}

	layer2Files := []FileApplier{
		newTestFile("/root/.bashrc", []byte("# Updated configuration"), 0644),
	}

	storage, err := newVFSStorage(filepath.Join(td, "storage-"))
	if err != nil {
		t.Fatal(err)
	}
	storageID1 := stringid.GenerateRandomID()
	storageID2 := stringid.GenerateRandomID()

	tar1, err := tarFromFilesInStorage(storage, storageID1, "", layer1Files...)
	if err != nil {
		t.Fatal(err)
	}

	tar2, err := tarFromFilesInStorage(storage, storageID2, storageID1, layer2Files...)
	if err != nil {
		t.Fatal(err)
	}

	fms, err := NewFSMetadataStore(filepath.Join(td, "layers"))
	if err != nil {
		t.Fatal(err)
	}
	ls, err := NewStoreFromStorage(fms, storage)
	if err != nil {
		t.Fatal(err)
	}

	newTarDataPath := filepath.Join(td, ".migration-tardata")
	diffID, size, err := ls.(*layerStore).ChecksumForStorageID(storageID1, "", "", newTarDataPath)
	if err != nil {
		t.Fatal(err)
	}

	layer1a, err := ls.(*layerStore).RegisterByStorageID(storageID1, "", diffID, newTarDataPath, size)
	if err != nil {
		t.Fatal(err)
	}

	layer1b, err := ls.Register(bytes.NewReader(tar1), "")
	if err != nil {
		t.Fatal(err)
	}

	assertReferences(t, layer1a, layer1b)

	// Attempt register, should be same
	layer2a, err := ls.Register(bytes.NewReader(tar2), layer1a.ChainID())
	if err != nil {
		t.Fatal(err)
	}

	diffID, size, err = ls.(*layerStore).ChecksumForStorageID(storageID2, storageID1, "", newTarDataPath)
	if err != nil {
		t.Fatal(err)
	}

	layer2b, err := ls.(*layerStore).RegisterByStorageID(storageID2, layer1a.ChainID(), diffID, newTarDataPath, size)
	if err != nil {
		t.Fatal(err)
	}
	assertReferences(t, layer2a, layer2b)

	if metadata, err := ls.Release(layer2a); err != nil {
		t.Fatal(err)
	} else if len(metadata) > 0 {
		t.Fatalf("Unexpected layer removal after first release: %#v", metadata)
	}

	metadata, err := ls.Release(layer2b)
	if err != nil {
		t.Fatal(err)
	}

	assertMetadata(t, metadata, createMetadata(layer2a))
}

func TestMountMigration(t *testing.T) {
	// TODO Windows: Figure out why this is failing (obvious - paths... needs porting)
	if runtime.GOOS == "windows" {
		t.Skip("Failing on Windows")
	}
	ls, _, cleanup := newTestStore(t)
	defer cleanup()

	baseFiles := []FileApplier{
		newTestFile("/root/.bashrc", []byte("# Boring configuration"), 0644),
		newTestFile("/etc/profile", []byte("# Base configuration"), 0644),
	}
	initFiles := []FileApplier{
		newTestFile("/etc/hosts", []byte{}, 0644),
		newTestFile("/etc/resolv.conf", []byte{}, 0644),
	}
	mountFiles := []FileApplier{
		newTestFile("/etc/hosts", []byte("localhost 127.0.0.1"), 0644),
		newTestFile("/root/.bashrc", []byte("# Updated configuration"), 0644),
		newTestFile("/root/testfile1.txt", []byte("nothing valuable"), 0644),
	}

	initTar, err := tarFromFiles(initFiles...)
	if err != nil {
		t.Fatal(err)
	}

	mountTar, err := tarFromFiles(mountFiles...)
	if err != nil {
		t.Fatal(err)
	}

	storage := ls.(*layerStore).driver

	layer1, err := createLayer(ls, "", initWithFiles(baseFiles...))
	if err != nil {
		t.Fatal(err)
	}

	storageID1 := layer1.(*referencedCacheLayer).cacheID

	containerID := stringid.GenerateRandomID()
	containerInit := fmt.Sprintf("%s-init", containerID)

	if err := storage.Create(containerInit, storageID1, "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.ApplyDiff(containerInit, storageID1, archive.Reader(bytes.NewReader(initTar))); err != nil {
		t.Fatal(err)
	}

	if err := storage.Create(containerID, containerInit, "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.ApplyDiff(containerID, containerInit, archive.Reader(bytes.NewReader(mountTar))); err != nil {
		t.Fatal(err)
	}

	if err := ls.(*layerStore).CreateRWLayerByStorageID("migration-mount", containerID, layer1.ChainID()); err != nil {
		t.Fatal(err)
	}

	rwLayer1, err := ls.GetRWLayer("migration-mount")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := rwLayer1.Mount(""); err != nil {
		t.Fatal(err)
	}

	changes, err := rwLayer1.Changes()
	if err != nil {
		t.Fatal(err)
	}

	if expected := 5; len(changes) != expected {
		t.Logf("Changes %#v", changes)
		t.Fatalf("Wrong number of changes %d, expected %d", len(changes), expected)
	}

	sortChanges(changes)

	assertChange(t, changes[0], archive.Change{
		Path: "/etc",
		Kind: archive.ChangeModify,
	})
	assertChange(t, changes[1], archive.Change{
		Path: "/etc/hosts",
		Kind: archive.ChangeModify,
	})
	assertChange(t, changes[2], archive.Change{
		Path: "/root",
		Kind: archive.ChangeModify,
	})
	assertChange(t, changes[3], archive.Change{
		Path: "/root/.bashrc",
		Kind: archive.ChangeModify,
	})
	assertChange(t, changes[4], archive.Change{
		Path: "/root/testfile1.txt",
		Kind: archive.ChangeAdd,
	})

	if _, err := ls.CreateRWLayer("migration-mount", layer1.ChainID(), "", nil, nil); err == nil {
		t.Fatal("Expected error creating mount with same name")
	} else if err != ErrMountNameConflict {
		t.Fatal(err)
	}

	rwLayer2, err := ls.GetRWLayer("migration-mount")
	if err != nil {
		t.Fatal(err)
	}

	if getMountLayer(rwLayer1) != getMountLayer(rwLayer2) {
		t.Fatal("Expected same layer from get with same name as from migrate")
	}

	if _, err := rwLayer2.Mount(""); err != nil {
		t.Fatal(err)
	}

	if _, err := rwLayer2.Mount(""); err != nil {
		t.Fatal(err)
	}

	if metadata, err := ls.Release(layer1); err != nil {
		t.Fatal(err)
	} else if len(metadata) > 0 {
		t.Fatalf("Expected no layers to be deleted, deleted %#v", metadata)
	}

	if err := rwLayer1.Unmount(); err != nil {
		t.Fatal(err)
	}

	if _, err := ls.ReleaseRWLayer(rwLayer1); err != nil {
		t.Fatal(err)
	}

	if err := rwLayer2.Unmount(); err != nil {
		t.Fatal(err)
	}
	if err := rwLayer2.Unmount(); err != nil {
		t.Fatal(err)
	}
	metadata, err := ls.ReleaseRWLayer(rwLayer2)
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata) == 0 {
		t.Fatal("Expected base layer to be deleted when deleting mount")
	}

	assertMetadata(t, metadata, createMetadata(layer1))
}
