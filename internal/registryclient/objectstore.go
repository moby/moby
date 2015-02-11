package client

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
)

var (
	// ErrLayerAlreadyExists is returned when attempting to create a layer with
	// a tarsum that is already in use.
	ErrLayerAlreadyExists = fmt.Errorf("Layer already exists")

	// ErrLayerLocked is returned when attempting to write to a layer which is
	// currently being written to.
	ErrLayerLocked = fmt.Errorf("Layer locked")
)

// ObjectStore is an interface which is designed to approximate the docker
// engine storage. This interface is subject to change to conform to the
// future requirements of the engine.
type ObjectStore interface {
	// Manifest retrieves the image manifest stored at the given repository name
	// and tag
	Manifest(name, tag string) (*manifest.SignedManifest, error)

	// WriteManifest stores an image manifest at the given repository name and
	// tag
	WriteManifest(name, tag string, manifest *manifest.SignedManifest) error

	// Layer returns a handle to a layer for reading and writing
	Layer(dgst digest.Digest) (Layer, error)
}

// Layer is a generic image layer interface.
// A Layer may not be written to if it is already complete.
type Layer interface {
	// Reader returns a LayerReader or an error if the layer has not been
	// written to or is currently being written to.
	Reader() (LayerReader, error)

	// Writer returns a LayerWriter or an error if the layer has been fully
	// written to or is currently being written to.
	Writer() (LayerWriter, error)

	// Wait blocks until the Layer can be read from.
	Wait() error
}

// LayerReader is a read-only handle to a Layer, which exposes the CurrentSize
// and full Size in addition to implementing the io.ReadCloser interface.
type LayerReader interface {
	io.ReadCloser

	// CurrentSize returns the number of bytes written to the underlying Layer
	CurrentSize() int

	// Size returns the full size of the underlying Layer
	Size() int
}

// LayerWriter is a write-only handle to a Layer, which exposes the CurrentSize
// and full Size in addition to implementing the io.WriteCloser interface.
// SetSize must be called on this LayerWriter before it can be written to.
type LayerWriter interface {
	io.WriteCloser

	// CurrentSize returns the number of bytes written to the underlying Layer
	CurrentSize() int

	// Size returns the full size of the underlying Layer
	Size() int

	// SetSize sets the full size of the underlying Layer.
	// This must be called before any calls to Write
	SetSize(int) error
}

// memoryObjectStore is an in-memory implementation of the ObjectStore interface
type memoryObjectStore struct {
	mutex           *sync.Mutex
	manifestStorage map[string]*manifest.SignedManifest
	layerStorage    map[digest.Digest]Layer
}

func (objStore *memoryObjectStore) Manifest(name, tag string) (*manifest.SignedManifest, error) {
	objStore.mutex.Lock()
	defer objStore.mutex.Unlock()

	manifest, ok := objStore.manifestStorage[name+":"+tag]
	if !ok {
		return nil, fmt.Errorf("No manifest found with Name: %q, Tag: %q", name, tag)
	}
	return manifest, nil
}

func (objStore *memoryObjectStore) WriteManifest(name, tag string, manifest *manifest.SignedManifest) error {
	objStore.mutex.Lock()
	defer objStore.mutex.Unlock()

	objStore.manifestStorage[name+":"+tag] = manifest
	return nil
}

func (objStore *memoryObjectStore) Layer(dgst digest.Digest) (Layer, error) {
	objStore.mutex.Lock()
	defer objStore.mutex.Unlock()

	layer, ok := objStore.layerStorage[dgst]
	if !ok {
		layer = &memoryLayer{cond: sync.NewCond(new(sync.Mutex))}
		objStore.layerStorage[dgst] = layer
	}

	return layer, nil
}

type memoryLayer struct {
	cond         *sync.Cond
	contents     []byte
	expectedSize int
	writing      bool
}

func (ml *memoryLayer) Reader() (LayerReader, error) {
	ml.cond.L.Lock()
	defer ml.cond.L.Unlock()

	if ml.contents == nil {
		return nil, fmt.Errorf("Layer has not been written to yet")
	}
	if ml.writing {
		return nil, ErrLayerLocked
	}

	return &memoryLayerReader{ml: ml, reader: bytes.NewReader(ml.contents)}, nil
}

func (ml *memoryLayer) Writer() (LayerWriter, error) {
	ml.cond.L.Lock()
	defer ml.cond.L.Unlock()

	if ml.contents != nil {
		if ml.writing {
			return nil, ErrLayerLocked
		}
		if ml.expectedSize == len(ml.contents) {
			return nil, ErrLayerAlreadyExists
		}
	} else {
		ml.contents = make([]byte, 0)
	}

	ml.writing = true
	return &memoryLayerWriter{ml: ml, buffer: bytes.NewBuffer(ml.contents)}, nil
}

func (ml *memoryLayer) Wait() error {
	ml.cond.L.Lock()
	defer ml.cond.L.Unlock()

	if ml.contents == nil {
		return fmt.Errorf("No writer to wait on")
	}

	for ml.writing {
		ml.cond.Wait()
	}

	return nil
}

type memoryLayerReader struct {
	ml     *memoryLayer
	reader *bytes.Reader
}

func (mlr *memoryLayerReader) Read(p []byte) (int, error) {
	return mlr.reader.Read(p)
}

func (mlr *memoryLayerReader) Close() error {
	return nil
}

func (mlr *memoryLayerReader) CurrentSize() int {
	return len(mlr.ml.contents)
}

func (mlr *memoryLayerReader) Size() int {
	return mlr.ml.expectedSize
}

type memoryLayerWriter struct {
	ml     *memoryLayer
	buffer *bytes.Buffer
}

func (mlw *memoryLayerWriter) Write(p []byte) (int, error) {
	if mlw.ml.expectedSize == 0 {
		return 0, fmt.Errorf("Must set size before writing to layer")
	}
	wrote, err := mlw.buffer.Write(p)
	mlw.ml.contents = mlw.buffer.Bytes()
	return wrote, err
}

func (mlw *memoryLayerWriter) Close() error {
	mlw.ml.cond.L.Lock()
	defer mlw.ml.cond.L.Unlock()

	return mlw.close()
}

func (mlw *memoryLayerWriter) close() error {
	mlw.ml.writing = false
	mlw.ml.cond.Broadcast()
	return nil
}

func (mlw *memoryLayerWriter) CurrentSize() int {
	return len(mlw.ml.contents)
}

func (mlw *memoryLayerWriter) Size() int {
	return mlw.ml.expectedSize
}

func (mlw *memoryLayerWriter) SetSize(size int) error {
	if !mlw.ml.writing {
		return fmt.Errorf("Layer is closed for writing")
	}
	mlw.ml.expectedSize = size
	return nil
}
