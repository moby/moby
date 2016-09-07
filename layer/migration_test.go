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

	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/stringid"
	"github.com/go-check/check"
	"github.com/vbatts/tar-split/tar/asm"
	"github.com/vbatts/tar-split/tar/storage"
)

func writeTarSplitFile(name string, tarContent []byte) error {
	f, err := os.OpenFile(name, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	fz := gzip.NewWriter(f)

	metaPacker := storage.NewJSONPacker(fz)
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

func (s *DockerSuite) TestLayerMigration(c *check.C) {
	// TODO Windows: Figure out why this is failing
	if runtime.GOOS == "windows" {
		c.Skip("Failing on Windows")
	}
	td, err := ioutil.TempDir("", "migration-test-")
	if err != nil {
		c.Fatal(err)
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
		c.Fatal(err)
	}

	tar2, err := tarFromFiles(layer2Files...)
	if err != nil {
		c.Fatal(err)
	}

	graph, err := newVFSGraphDriver(filepath.Join(td, "graphdriver-"))
	if err != nil {
		c.Fatal(err)
	}

	graphID1 := stringid.GenerateRandomID()
	if err := graph.Create(graphID1, "", "", nil); err != nil {
		c.Fatal(err)
	}
	if _, err := graph.ApplyDiff(graphID1, "", archive.Reader(bytes.NewReader(tar1))); err != nil {
		c.Fatal(err)
	}

	tf1 := filepath.Join(td, "tar1.json.gz")
	if err := writeTarSplitFile(tf1, tar1); err != nil {
		c.Fatal(err)
	}

	fms, err := NewFSMetadataStore(filepath.Join(td, "layers"))
	if err != nil {
		c.Fatal(err)
	}
	ls, err := NewStoreFromGraphDriver(fms, graph)
	if err != nil {
		c.Fatal(err)
	}

	newTarDataPath := filepath.Join(td, ".migration-tardata")
	diffID, size, err := ls.(*layerStore).ChecksumForGraphID(graphID1, "", tf1, newTarDataPath)
	if err != nil {
		c.Fatal(err)
	}

	layer1a, err := ls.(*layerStore).RegisterByGraphID(graphID1, "", diffID, newTarDataPath, size)
	if err != nil {
		c.Fatal(err)
	}

	layer1b, err := ls.Register(bytes.NewReader(tar1), "")
	if err != nil {
		c.Fatal(err)
	}

	assertReferences(c, layer1a, layer1b)
	// Attempt register, should be same
	layer2a, err := ls.Register(bytes.NewReader(tar2), layer1a.ChainID())
	if err != nil {
		c.Fatal(err)
	}

	graphID2 := stringid.GenerateRandomID()
	if err := graph.Create(graphID2, graphID1, "", nil); err != nil {
		c.Fatal(err)
	}
	if _, err := graph.ApplyDiff(graphID2, graphID1, archive.Reader(bytes.NewReader(tar2))); err != nil {
		c.Fatal(err)
	}

	tf2 := filepath.Join(td, "tar2.json.gz")
	if err := writeTarSplitFile(tf2, tar2); err != nil {
		c.Fatal(err)
	}
	diffID, size, err = ls.(*layerStore).ChecksumForGraphID(graphID2, graphID1, tf2, newTarDataPath)
	if err != nil {
		c.Fatal(err)
	}

	layer2b, err := ls.(*layerStore).RegisterByGraphID(graphID2, layer1a.ChainID(), diffID, tf2, size)
	if err != nil {
		c.Fatal(err)
	}
	assertReferences(c, layer2a, layer2b)

	if metadata, err := ls.Release(layer2a); err != nil {
		c.Fatal(err)
	} else if len(metadata) > 0 {
		c.Fatalf("Unexpected layer removal after first release: %#v", metadata)
	}

	metadata, err := ls.Release(layer2b)
	if err != nil {
		c.Fatal(err)
	}

	assertMetadata(c, metadata, createMetadata(layer2a))
}

func tarFromFilesInGraph(graph graphdriver.Driver, graphID, parentID string, files ...FileApplier) ([]byte, error) {
	t, err := tarFromFiles(files...)
	if err != nil {
		return nil, err
	}

	if err := graph.Create(graphID, parentID, "", nil); err != nil {
		return nil, err
	}
	if _, err := graph.ApplyDiff(graphID, parentID, archive.Reader(bytes.NewReader(t))); err != nil {
		return nil, err
	}

	ar, err := graph.Diff(graphID, parentID)
	if err != nil {
		return nil, err
	}
	defer ar.Close()

	return ioutil.ReadAll(ar)
}

func (s *DockerSuite) TestLayerMigrationNoTarsplit(c *check.C) {
	// TODO Windows: Figure out why this is failing
	if runtime.GOOS == "windows" {
		c.Skip("Failing on Windows")
	}
	td, err := ioutil.TempDir("", "migration-test-")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(td)

	layer1Files := []FileApplier{
		newTestFile("/root/.bashrc", []byte("# Boring configuration"), 0644),
		newTestFile("/etc/profile", []byte("# Base configuration"), 0644),
	}

	layer2Files := []FileApplier{
		newTestFile("/root/.bashrc", []byte("# Updated configuration"), 0644),
	}

	graph, err := newVFSGraphDriver(filepath.Join(td, "graphdriver-"))
	if err != nil {
		c.Fatal(err)
	}
	graphID1 := stringid.GenerateRandomID()
	graphID2 := stringid.GenerateRandomID()

	tar1, err := tarFromFilesInGraph(graph, graphID1, "", layer1Files...)
	if err != nil {
		c.Fatal(err)
	}

	tar2, err := tarFromFilesInGraph(graph, graphID2, graphID1, layer2Files...)
	if err != nil {
		c.Fatal(err)
	}

	fms, err := NewFSMetadataStore(filepath.Join(td, "layers"))
	if err != nil {
		c.Fatal(err)
	}
	ls, err := NewStoreFromGraphDriver(fms, graph)
	if err != nil {
		c.Fatal(err)
	}

	newTarDataPath := filepath.Join(td, ".migration-tardata")
	diffID, size, err := ls.(*layerStore).ChecksumForGraphID(graphID1, "", "", newTarDataPath)
	if err != nil {
		c.Fatal(err)
	}

	layer1a, err := ls.(*layerStore).RegisterByGraphID(graphID1, "", diffID, newTarDataPath, size)
	if err != nil {
		c.Fatal(err)
	}

	layer1b, err := ls.Register(bytes.NewReader(tar1), "")
	if err != nil {
		c.Fatal(err)
	}

	assertReferences(c, layer1a, layer1b)

	// Attempt register, should be same
	layer2a, err := ls.Register(bytes.NewReader(tar2), layer1a.ChainID())
	if err != nil {
		c.Fatal(err)
	}

	diffID, size, err = ls.(*layerStore).ChecksumForGraphID(graphID2, graphID1, "", newTarDataPath)
	if err != nil {
		c.Fatal(err)
	}

	layer2b, err := ls.(*layerStore).RegisterByGraphID(graphID2, layer1a.ChainID(), diffID, newTarDataPath, size)
	if err != nil {
		c.Fatal(err)
	}
	assertReferences(c, layer2a, layer2b)

	if metadata, err := ls.Release(layer2a); err != nil {
		c.Fatal(err)
	} else if len(metadata) > 0 {
		c.Fatalf("Unexpected layer removal after first release: %#v", metadata)
	}

	metadata, err := ls.Release(layer2b)
	if err != nil {
		c.Fatal(err)
	}

	assertMetadata(c, metadata, createMetadata(layer2a))
}

func (s *DockerSuite) TestMountMigration(c *check.C) {
	// TODO Windows: Figure out why this is failing (obvious - paths... needs porting)
	if runtime.GOOS == "windows" {
		c.Skip("Failing on Windows")
	}
	ls, _, cleanup := newTestStore(c)
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
		c.Fatal(err)
	}

	mountTar, err := tarFromFiles(mountFiles...)
	if err != nil {
		c.Fatal(err)
	}

	graph := ls.(*layerStore).driver

	layer1, err := createLayer(ls, "", initWithFiles(baseFiles...))
	if err != nil {
		c.Fatal(err)
	}

	graphID1 := layer1.(*referencedCacheLayer).cacheID

	containerID := stringid.GenerateRandomID()
	containerInit := fmt.Sprintf("%s-init", containerID)

	if err := graph.Create(containerInit, graphID1, "", nil); err != nil {
		c.Fatal(err)
	}
	if _, err := graph.ApplyDiff(containerInit, graphID1, archive.Reader(bytes.NewReader(initTar))); err != nil {
		c.Fatal(err)
	}

	if err := graph.Create(containerID, containerInit, "", nil); err != nil {
		c.Fatal(err)
	}
	if _, err := graph.ApplyDiff(containerID, containerInit, archive.Reader(bytes.NewReader(mountTar))); err != nil {
		c.Fatal(err)
	}

	if err := ls.(*layerStore).CreateRWLayerByGraphID("migration-mount", containerID, layer1.ChainID()); err != nil {
		c.Fatal(err)
	}

	rwLayer1, err := ls.GetRWLayer("migration-mount")
	if err != nil {
		c.Fatal(err)
	}

	if _, err := rwLayer1.Mount(""); err != nil {
		c.Fatal(err)
	}

	changes, err := rwLayer1.Changes()
	if err != nil {
		c.Fatal(err)
	}

	if expected := 5; len(changes) != expected {
		c.Logf("Changes %#v", changes)
		c.Fatalf("Wrong number of changes %d, expected %d", len(changes), expected)
	}

	sortChanges(changes)

	assertChange(c, changes[0], archive.Change{
		Path: "/etc",
		Kind: archive.ChangeModify,
	})
	assertChange(c, changes[1], archive.Change{
		Path: "/etc/hosts",
		Kind: archive.ChangeModify,
	})
	assertChange(c, changes[2], archive.Change{
		Path: "/root",
		Kind: archive.ChangeModify,
	})
	assertChange(c, changes[3], archive.Change{
		Path: "/root/.bashrc",
		Kind: archive.ChangeModify,
	})
	assertChange(c, changes[4], archive.Change{
		Path: "/root/testfile1.txt",
		Kind: archive.ChangeAdd,
	})

	if _, err := ls.CreateRWLayer("migration-mount", layer1.ChainID(), "", nil, nil); err == nil {
		c.Fatal("Expected error creating mount with same name")
	} else if err != ErrMountNameConflict {
		c.Fatal(err)
	}

	rwLayer2, err := ls.GetRWLayer("migration-mount")
	if err != nil {
		c.Fatal(err)
	}

	if getMountLayer(rwLayer1) != getMountLayer(rwLayer2) {
		c.Fatal("Expected same layer from get with same name as from migrate")
	}

	if _, err := rwLayer2.Mount(""); err != nil {
		c.Fatal(err)
	}

	if _, err := rwLayer2.Mount(""); err != nil {
		c.Fatal(err)
	}

	if metadata, err := ls.Release(layer1); err != nil {
		c.Fatal(err)
	} else if len(metadata) > 0 {
		c.Fatalf("Expected no layers to be deleted, deleted %#v", metadata)
	}

	if err := rwLayer1.Unmount(); err != nil {
		c.Fatal(err)
	}

	if _, err := ls.ReleaseRWLayer(rwLayer1); err != nil {
		c.Fatal(err)
	}

	if err := rwLayer2.Unmount(); err != nil {
		c.Fatal(err)
	}
	if err := rwLayer2.Unmount(); err != nil {
		c.Fatal(err)
	}
	metadata, err := ls.ReleaseRWLayer(rwLayer2)
	if err != nil {
		c.Fatal(err)
	}
	if len(metadata) == 0 {
		c.Fatal("Expected base layer to be deleted when deleting mount")
	}

	assertMetadata(c, metadata, createMetadata(layer1))
}
