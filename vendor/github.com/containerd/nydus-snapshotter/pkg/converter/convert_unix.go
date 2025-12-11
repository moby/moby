//go:build !windows
// +build !windows

/*
 * Copyright (c) 2022. Nydus Developers. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package converter

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/images/converter"
	"github.com/containerd/containerd/v2/pkg/archive"
	"github.com/containerd/containerd/v2/pkg/archive/compression"
	"github.com/containerd/containerd/v2/pkg/labels"
	"github.com/containerd/containerd/v2/plugins/content/local"
	"github.com/containerd/errdefs"
	"github.com/containerd/fifo"
	"github.com/klauspost/compress/zstd"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/containerd/nydus-snapshotter/pkg/converter/tool"
	"github.com/containerd/nydus-snapshotter/pkg/label"
)

const EntryBlob = "image.blob"
const EntryBootstrap = "image.boot"
const EntryBlobMeta = "blob.meta"
const EntryBlobMetaHeader = "blob.meta.header"
const EntryTOC = "rafs.blob.toc"

const envNydusBuilder = "NYDUS_BUILDER"
const envNydusWorkDir = "NYDUS_WORKDIR"

const configGCLabelKey = "containerd.io/gc.ref.content.config"

var bufPool = sync.Pool{
	New: func() interface{} {
		buffer := make([]byte, 1<<20)
		return &buffer
	},
}

func getBuilder(specifiedPath string) string {
	if specifiedPath != "" {
		return specifiedPath
	}

	builderPath := os.Getenv(envNydusBuilder)
	if builderPath != "" {
		return builderPath
	}

	return "nydus-image"
}

func ensureWorkDir(specifiedBasePath string) (string, error) {
	var baseWorkDir string

	if specifiedBasePath != "" {
		baseWorkDir = specifiedBasePath
	} else {
		baseWorkDir = os.Getenv(envNydusWorkDir)
	}
	if baseWorkDir == "" {
		baseWorkDir = os.TempDir()
	}

	if err := os.MkdirAll(baseWorkDir, 0750); err != nil {
		return "", errors.Wrapf(err, "create base directory %s", baseWorkDir)
	}

	workDirPath, err := os.MkdirTemp(baseWorkDir, "nydus-converter-")
	if err != nil {
		return "", errors.Wrap(err, "create work directory")
	}

	return workDirPath, nil
}

// Unpack a OCI formatted tar stream into a directory.
func unpackOciTar(ctx context.Context, dst string, reader io.Reader) error {
	ds, err := compression.DecompressStream(reader)
	if err != nil {
		return errors.Wrap(err, "unpack stream")
	}
	defer ds.Close()

	if _, err := archive.Apply(
		ctx,
		dst,
		ds,
		archive.WithConvertWhiteout(func(_ *tar.Header, _ string) (bool, error) {
			// Keep to extract all whiteout files.
			return true, nil
		}),
	); err != nil {
		return errors.Wrap(err, "apply with convert whiteout")
	}

	// Read any trailing data for some tar formats, in case the
	// PipeWriter of opposite side gets stuck.
	if _, err := io.Copy(io.Discard, ds); err != nil {
		return errors.Wrap(err, "trailing data after applying archive")
	}

	return nil
}

// unpackNydusBlob unpacks a Nydus formatted tar stream into a directory.
// unpackBlob indicates whether to unpack blob data.
func unpackNydusBlob(bootDst, blobDst string, ra content.ReaderAt, unpackBlob bool) error {
	boot, err := os.OpenFile(bootDst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		return errors.Wrapf(err, "write to bootstrap %s", bootDst)
	}
	defer boot.Close()

	if _, err = UnpackEntry(ra, EntryBootstrap, boot); err != nil {
		return errors.Wrap(err, "unpack bootstrap from nydus")
	}

	if unpackBlob {
		blob, err := os.OpenFile(blobDst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
		if err != nil {
			return errors.Wrapf(err, "write to blob %s", blobDst)
		}
		defer blob.Close()

		if _, err = UnpackEntry(ra, EntryBlob, blob); err != nil {
			if errors.Is(err, ErrNotFound) {
				// The nydus layer may contain only bootstrap and no blob
				// data, which should be ignored.
				return nil
			}
			return errors.Wrap(err, "unpack blob from nydus")
		}
	}

	return nil
}

func seekFileByTarHeader(ra content.ReaderAt, targetName string, maxSize *int64, handle func(io.Reader, *tar.Header) error) error {
	const headerSize = 512

	if headerSize > ra.Size() {
		return fmt.Errorf("invalid nydus tar size %d", ra.Size())
	}

	cur := ra.Size() - headerSize
	reader := newSeekReader(ra)

	// Seek from tail to head of nydus formatted tar stream to find
	// target data.
	for {
		// Try to seek the part of tar header.
		_, err := reader.Seek(cur, io.SeekStart)
		if err != nil {
			return errors.Wrapf(err, "seek %d for nydus tar header", cur)
		}

		// Parse tar header.
		tr := tar.NewReader(reader)
		hdr, err := tr.Next()
		if err != nil {
			return errors.Wrap(err, "parse nydus tar header")
		}

		if cur < hdr.Size {
			return fmt.Errorf("invalid nydus tar data, name %s, size %d", hdr.Name, hdr.Size)
		}

		if hdr.Name == targetName {
			if maxSize != nil && hdr.Size > *maxSize {
				return fmt.Errorf("invalid nydus tar size %d", ra.Size())
			}

			// Try to seek the part of tar data.
			_, err = reader.Seek(cur-hdr.Size, io.SeekStart)
			if err != nil {
				return errors.Wrap(err, "seek target data offset")
			}
			dataReader := io.NewSectionReader(reader, cur-hdr.Size, hdr.Size)

			if err := handle(dataReader, hdr); err != nil {
				return errors.Wrap(err, "handle target data")
			}

			return nil
		}

		cur = cur - hdr.Size - headerSize
		if cur < 0 {
			break
		}
	}

	return errors.Wrapf(ErrNotFound, "can't find target %s by seeking tar", targetName)
}

func seekFileByTOC(ra content.ReaderAt, targetName string, handle func(io.Reader, *tar.Header) error) (*TOCEntry, error) {
	entrySize := 128
	maxSize := int64(1 << 20)
	var tocEntry *TOCEntry

	err := seekFileByTarHeader(ra, EntryTOC, &maxSize, func(tocEntryDataReader io.Reader, _ *tar.Header) error {
		entryData, err := io.ReadAll(tocEntryDataReader)
		if err != nil {
			return errors.Wrap(err, "read toc entries")
		}
		if len(entryData)%entrySize != 0 {
			return fmt.Errorf("invalid entries length %d", len(entryData))
		}

		count := len(entryData) / entrySize
		for i := 0; i < count; i++ {
			var entry TOCEntry
			r := bytes.NewReader(entryData[i*entrySize : i*entrySize+entrySize])
			if err := binary.Read(r, binary.LittleEndian, &entry); err != nil {
				return errors.Wrap(err, "read toc entries")
			}
			if entry.GetName() == targetName {
				compressor, err := entry.GetCompressor()
				if err != nil {
					return errors.Wrap(err, "get compressor of entry")
				}
				compressedOffset := int64(entry.GetCompressedOffset())
				compressedSize := int64(entry.GetCompressedSize())
				sr := io.NewSectionReader(ra, compressedOffset, compressedSize)

				var rd io.Reader
				switch compressor {
				case CompressorZstd:
					decoder, err := zstd.NewReader(sr)
					if err != nil {
						return errors.Wrap(err, "seek to target data offset")
					}
					defer decoder.Close()
					rd = decoder
				case CompressorNone:
					rd = sr
				default:
					return fmt.Errorf("unsupported compressor %x", compressor)
				}

				if err := handle(rd, nil); err != nil {
					return errors.Wrap(err, "handle target entry data")
				}

				tocEntry = &entry

				return nil
			}
		}

		return errors.Wrapf(ErrNotFound, "can't find target %s by seeking TOC", targetName)
	})

	return tocEntry, err
}

// Unpack the file from nydus formatted tar stream.
// The nydus formatted tar stream is a tar-like structure that arranges the
// data as follows:
//
// `data | tar_header | ... | data | tar_header | [toc_entry | ... | toc_entry | tar_header]`
func UnpackEntry(ra content.ReaderAt, targetName string, target io.Writer) (*TOCEntry, error) {
	handle := func(dataReader io.Reader, _ *tar.Header) error {
		// Copy data to provided target writer.
		if _, err := io.Copy(target, dataReader); err != nil {
			return errors.Wrap(err, "copy target data to reader")
		}

		return nil
	}

	return seekFile(ra, targetName, handle)
}

func seekFile(ra content.ReaderAt, targetName string, handle func(io.Reader, *tar.Header) error) (*TOCEntry, error) {
	// Try seek target data by TOC.
	entry, err := seekFileByTOC(ra, targetName, handle)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return nil, errors.Wrap(err, "seek file by TOC")
		}
	} else {
		return entry, nil
	}

	// Seek target data by tar header, ensure compatible with old rafs blob format.
	return nil, seekFileByTarHeader(ra, targetName, nil, handle)
}

// Pack converts an OCI tar stream to nydus formatted stream with a tar-like
// structure that arranges the data as follows:
//
// `data | tar_header | data | tar_header | [toc_entry | ... | toc_entry | tar_header]`
//
// The caller should write OCI tar stream into the returned `io.WriteCloser`,
// then the Pack method will write the nydus formatted stream to `dest`
// provided by the caller.
//
// Important: the caller must check `io.WriteCloser.Close() == nil` to ensure
// the conversion workflow is finished.
func Pack(ctx context.Context, dest io.Writer, opt PackOption) (io.WriteCloser, error) {
	if opt.FsVersion == "" {
		opt.FsVersion = "6"
	}

	builderPath := getBuilder(opt.BuilderPath)

	requiredFeatures := tool.NewFeatures(tool.FeatureTar2Rafs)
	if opt.BatchSize != "" && opt.BatchSize != "0" {
		requiredFeatures.Add(tool.FeatureBatchSize)
	}
	if opt.Encrypt {
		requiredFeatures.Add(tool.FeatureEncrypt)
	}

	detectedFeatures, err := tool.DetectFeatures(builderPath, requiredFeatures, tool.GetHelp)
	if err != nil {
		return nil, err
	}
	opt.features = detectedFeatures

	if opt.OCIRef {
		if opt.FsVersion == "6" {
			return packFromTar(ctx, dest, opt)
		}
		return nil, fmt.Errorf("oci ref can only be supported by fs version 6")
	}

	if opt.features.Contains(tool.FeatureBatchSize) && opt.FsVersion != "6" {
		return nil, fmt.Errorf("'--batch-size' can only be supported by fs version 6")
	}

	if opt.features.Contains(tool.FeatureTar2Rafs) {
		return packFromTar(ctx, dest, opt)
	}

	return packFromDirectory(ctx, dest, opt, builderPath)
}

func packFromDirectory(ctx context.Context, dest io.Writer, opt PackOption, builderPath string) (io.WriteCloser, error) {
	workDir, err := ensureWorkDir(opt.WorkDir)
	if err != nil {
		return nil, errors.Wrap(err, "ensure work directory")
	}
	defer func() {
		if err != nil {
			os.RemoveAll(workDir)
		}
	}()

	sourceDir := filepath.Join(workDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return nil, errors.Wrap(err, "create source directory")
	}

	pr, pw := io.Pipe()

	unpackDone := make(chan bool, 1)
	go func() {
		if err := unpackOciTar(ctx, sourceDir, pr); err != nil {
			pr.CloseWithError(errors.Wrapf(err, "unpack to %s", sourceDir))
			close(unpackDone)
			return
		}
		unpackDone <- true
	}()

	wc := newWriteCloser(pw, func() error {
		defer os.RemoveAll(workDir)

		// Because PipeWriter#Close is called does not mean that the PipeReader
		// has finished reading all the data, and unpack may not be complete yet,
		// so we need to wait for that here.
		<-unpackDone

		blobPath := filepath.Join(workDir, "blob")
		blobFifo, err := fifo.OpenFifo(ctx, blobPath, syscall.O_CREAT|syscall.O_RDONLY|syscall.O_NONBLOCK, 0640)
		if err != nil {
			return errors.Wrapf(err, "create fifo file")
		}
		defer blobFifo.Close()

		go func() {
			err := tool.Pack(tool.PackOption{
				BuilderPath: builderPath,

				BlobPath:         blobPath,
				FsVersion:        opt.FsVersion,
				SourcePath:       sourceDir,
				ChunkDictPath:    opt.ChunkDictPath,
				PrefetchPatterns: opt.PrefetchPatterns,
				AlignedChunk:     opt.AlignedChunk,
				ChunkSize:        opt.ChunkSize,
				BatchSize:        opt.BatchSize,
				Compressor:       opt.Compressor,
				Timeout:          opt.Timeout,
				Encrypt:          opt.Encrypt,

				Features: opt.features,
			})
			if err != nil {
				pw.CloseWithError(errors.Wrapf(err, "convert blob for %s", sourceDir))
				blobFifo.Close()
			}
		}()

		buffer := bufPool.Get().(*[]byte)
		defer bufPool.Put(buffer)
		if _, err := io.CopyBuffer(dest, blobFifo, *buffer); err != nil {
			return errors.Wrap(err, "pack nydus tar")
		}

		return nil
	})

	return wc, nil
}

func packFromTar(ctx context.Context, dest io.Writer, opt PackOption) (io.WriteCloser, error) {
	workDir, err := ensureWorkDir(opt.WorkDir)
	if err != nil {
		return nil, errors.Wrap(err, "ensure work directory")
	}
	defer func() {
		if err != nil {
			os.RemoveAll(workDir)
		}
	}()

	rafsBlobPath := filepath.Join(workDir, "blob.rafs")
	rafsBlobFifo, err := fifo.OpenFifo(ctx, rafsBlobPath, syscall.O_CREAT|syscall.O_RDONLY|syscall.O_NONBLOCK, 0640)
	if err != nil {
		return nil, errors.Wrapf(err, "create fifo file")
	}

	tarBlobPath := filepath.Join(workDir, "blob.targz")
	tarBlobFifo, err := fifo.OpenFifo(ctx, tarBlobPath, syscall.O_CREAT|syscall.O_WRONLY|syscall.O_NONBLOCK, 0640)
	if err != nil {
		defer rafsBlobFifo.Close()
		return nil, errors.Wrapf(err, "create fifo file")
	}

	pr, pw := io.Pipe()
	eg := errgroup.Group{}

	wc := newWriteCloser(pw, func() error {
		defer os.RemoveAll(workDir)
		if err := eg.Wait(); err != nil {
			return errors.Wrapf(err, "convert nydus ref")
		}
		return nil
	})

	eg.Go(func() error {
		defer tarBlobFifo.Close()
		buffer := bufPool.Get().(*[]byte)
		defer bufPool.Put(buffer)
		if _, err := io.CopyBuffer(tarBlobFifo, pr, *buffer); err != nil {
			return errors.Wrapf(err, "copy targz to fifo")
		}
		return nil
	})

	eg.Go(func() error {
		defer rafsBlobFifo.Close()
		buffer := bufPool.Get().(*[]byte)
		defer bufPool.Put(buffer)
		if _, err := io.CopyBuffer(dest, rafsBlobFifo, *buffer); err != nil {
			return errors.Wrapf(err, "copy blob meta fifo to nydus blob")
		}
		return nil
	})

	eg.Go(func() error {
		var err error
		if opt.OCIRef {
			err = tool.Pack(tool.PackOption{
				BuilderPath: getBuilder(opt.BuilderPath),

				OCIRef:     opt.OCIRef,
				BlobPath:   rafsBlobPath,
				SourcePath: tarBlobPath,
				Timeout:    opt.Timeout,

				Features: opt.features,
			})
		} else {
			err = tool.Pack(tool.PackOption{
				BuilderPath: getBuilder(opt.BuilderPath),

				BlobPath:         rafsBlobPath,
				FsVersion:        opt.FsVersion,
				SourcePath:       tarBlobPath,
				ChunkDictPath:    opt.ChunkDictPath,
				PrefetchPatterns: opt.PrefetchPatterns,
				AlignedChunk:     opt.AlignedChunk,
				ChunkSize:        opt.ChunkSize,
				BatchSize:        opt.BatchSize,
				Compressor:       opt.Compressor,
				Timeout:          opt.Timeout,
				Encrypt:          opt.Encrypt,

				Features: opt.features,
			})
		}
		if err != nil {
			// Without handling the returned error because we just only
			// focus on the command exit status in `tool.Pack`.
			wc.Close()
		}
		return errors.Wrapf(err, "call builder")
	})

	return wc, nil
}

func calcBlobTOCDigest(ra content.ReaderAt) (*digest.Digest, error) {
	maxSize := int64(1 << 20)
	digester := digest.Canonical.Digester()
	if err := seekFileByTarHeader(ra, EntryTOC, &maxSize, func(tocData io.Reader, _ *tar.Header) error {
		if _, err := io.Copy(digester.Hash(), tocData); err != nil {
			return errors.Wrap(err, "calc toc data and header digest")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	tocDigest := digester.Digest()
	return &tocDigest, nil
}

// Merge multiple nydus bootstraps (from each layer of image) to a final
// bootstrap. And due to the possibility of enabling the `ChunkDictPath`
// option causes the data deduplication, it will return the actual blob
// digests referenced by the bootstrap.
func Merge(ctx context.Context, layers []Layer, dest io.Writer, opt MergeOption) ([]digest.Digest, error) {
	workDir, err := ensureWorkDir(opt.WorkDir)
	if err != nil {
		return nil, errors.Wrap(err, "ensure work directory")
	}
	defer os.RemoveAll(workDir)

	getBootstrapPath := func(layerIdx int) string {
		digestHex := layers[layerIdx].Digest.Hex()
		if originalDigest := layers[layerIdx].OriginalDigest; originalDigest != nil {
			return filepath.Join(workDir, originalDigest.Hex())
		}
		return filepath.Join(workDir, digestHex)
	}

	eg, _ := errgroup.WithContext(ctx)
	sourceBootstrapPaths := []string{}
	rafsBlobDigests := []string{}
	rafsBlobSizes := []int64{}
	rafsBlobTOCDigests := []string{}
	for idx := range layers {
		sourceBootstrapPaths = append(sourceBootstrapPaths, getBootstrapPath(idx))
		if layers[idx].OriginalDigest != nil {
			rafsBlobTOCDigest, err := calcBlobTOCDigest(layers[idx].ReaderAt)
			if err != nil {
				return nil, errors.Wrapf(err, "calc blob toc digest for layer %s", layers[idx].Digest)
			}
			rafsBlobTOCDigests = append(rafsBlobTOCDigests, rafsBlobTOCDigest.Hex())
			rafsBlobDigests = append(rafsBlobDigests, layers[idx].Digest.Hex())
			rafsBlobSizes = append(rafsBlobSizes, layers[idx].ReaderAt.Size())
		}
		eg.Go(func(idx int) func() error {
			return func() error {
				// Use the hex hash string of whole tar blob as the bootstrap name.
				bootstrap, err := os.Create(getBootstrapPath(idx))
				if err != nil {
					return errors.Wrap(err, "create source bootstrap")
				}
				defer bootstrap.Close()

				if _, err := UnpackEntry(layers[idx].ReaderAt, EntryBootstrap, bootstrap); err != nil {
					return errors.Wrap(err, "unpack nydus tar")
				}

				return nil
			}
		}(idx))
	}

	if err := eg.Wait(); err != nil {
		return nil, errors.Wrap(err, "unpack all bootstraps")
	}

	targetBootstrapPath := filepath.Join(workDir, "bootstrap")

	blobDigests, err := tool.Merge(tool.MergeOption{
		BuilderPath: getBuilder(opt.BuilderPath),

		SourceBootstrapPaths: sourceBootstrapPaths,
		RafsBlobDigests:      rafsBlobDigests,
		RafsBlobSizes:        rafsBlobSizes,
		RafsBlobTOCDigests:   rafsBlobTOCDigests,

		TargetBootstrapPath: targetBootstrapPath,
		ChunkDictPath:       opt.ChunkDictPath,
		ParentBootstrapPath: opt.ParentBootstrapPath,
		PrefetchPatterns:    opt.PrefetchPatterns,
		OutputJSONPath:      filepath.Join(workDir, "merge-output.json"),
		Timeout:             opt.Timeout,
	})
	if err != nil {
		return nil, errors.Wrap(err, "merge bootstrap")
	}

	bootstrapRa, err := local.OpenReader(targetBootstrapPath)
	if err != nil {
		return nil, errors.Wrap(err, "open bootstrap reader")
	}
	defer bootstrapRa.Close()

	files := append([]File{
		{
			Name:   EntryBootstrap,
			Reader: content.NewReader(bootstrapRa),
			Size:   bootstrapRa.Size(),
		},
	}, opt.AppendFiles...)
	var rc io.ReadCloser

	if opt.WithTar {
		rc = packToTar(files, false)
	} else {
		rc, err = os.Open(targetBootstrapPath)
		if err != nil {
			return nil, errors.Wrap(err, "open targe bootstrap")
		}
	}
	defer rc.Close()

	buffer := bufPool.Get().(*[]byte)
	defer bufPool.Put(buffer)
	if _, err = io.CopyBuffer(dest, rc, *buffer); err != nil {
		return nil, errors.Wrap(err, "copy merged bootstrap")
	}

	return blobDigests, nil
}

// Unpack converts a nydus blob layer to OCI formatted tar stream.
func Unpack(ctx context.Context, ra content.ReaderAt, dest io.Writer, opt UnpackOption) error {
	workDir, err := ensureWorkDir(opt.WorkDir)
	if err != nil {
		return errors.Wrap(err, "ensure work directory")
	}
	defer os.RemoveAll(workDir)

	bootPath, blobPath := filepath.Join(workDir, EntryBootstrap), filepath.Join(workDir, EntryBlob)
	if err = unpackNydusBlob(bootPath, blobPath, ra, !opt.Stream); err != nil {
		return errors.Wrap(err, "unpack nydus tar")
	}

	tarPath := filepath.Join(workDir, "oci.tar")
	blobFifo, err := fifo.OpenFifo(ctx, tarPath, syscall.O_CREAT|syscall.O_RDONLY|syscall.O_NONBLOCK, 0640)
	if err != nil {
		return errors.Wrapf(err, "create fifo file")
	}
	defer blobFifo.Close()

	unpackOpt := tool.UnpackOption{
		BuilderPath:   getBuilder(opt.BuilderPath),
		BootstrapPath: bootPath,
		BlobPath:      blobPath,
		TarPath:       tarPath,
		Timeout:       opt.Timeout,
	}

	if opt.Stream {
		proxy, err := setupContentStoreProxy(opt.WorkDir, ra)
		if err != nil {
			return errors.Wrap(err, "new content store proxy")
		}
		defer proxy.close()

		// generate backend config file
		backendConfigStr := fmt.Sprintf(`{"version":2,"backend":{"type":"http-proxy","http-proxy":{"addr":"%s"}}}`, proxy.socketPath)
		backendConfigPath := filepath.Join(workDir, "backend-config.json")
		if err := os.WriteFile(backendConfigPath, []byte(backendConfigStr), 0640); err != nil {
			return errors.Wrap(err, "write backend config")
		}
		unpackOpt.BlobPath = ""
		unpackOpt.BackendConfigPath = backendConfigPath
	}

	unpackErrChan := make(chan error)
	go func() {
		defer close(unpackErrChan)
		err := tool.Unpack(unpackOpt)
		if err != nil {
			blobFifo.Close()
			unpackErrChan <- err
		}
	}()

	buffer := bufPool.Get().(*[]byte)
	defer bufPool.Put(buffer)
	if _, err := io.CopyBuffer(dest, blobFifo, *buffer); err != nil {
		if unpackErr := <-unpackErrChan; unpackErr != nil {
			return errors.Wrap(unpackErr, "unpack")
		}
		return errors.Wrap(err, "copy oci tar")
	}

	return nil
}

// IsNydusBlobAndExists returns true when the specified digest of content exists in
// the content store and it's nydus blob format.
func IsNydusBlobAndExists(ctx context.Context, cs content.Store, desc ocispec.Descriptor) bool {
	_, err := cs.Info(ctx, desc.Digest)
	if err != nil {
		return false
	}

	return IsNydusBlob(desc)
}

// IsNydusBlob returns true when the specified descriptor is nydus blob layer.
func IsNydusBlob(desc ocispec.Descriptor) bool {
	if desc.Annotations == nil {
		return false
	}

	_, hasAnno := desc.Annotations[LayerAnnotationNydusBlob]
	return hasAnno
}

// IsNydusBootstrap returns true when the specified descriptor is nydus bootstrap layer.
func IsNydusBootstrap(desc ocispec.Descriptor) bool {
	if desc.Annotations == nil {
		return false
	}

	_, hasAnno := desc.Annotations[LayerAnnotationNydusBootstrap]
	return hasAnno
}

// isNydusImage checks if the last layer is nydus bootstrap,
// so that we can ensure it is a nydus image.
func isNydusImage(manifest *ocispec.Manifest) bool {
	layers := manifest.Layers
	if len(layers) != 0 {
		desc := layers[len(layers)-1]
		if IsNydusBootstrap(desc) {
			return true
		}
	}
	return false
}

// makeBlobDesc returns a ocispec.Descriptor by the given information.
func makeBlobDesc(ctx context.Context, cs content.Store, opt PackOption, sourceDigest, targetDigest digest.Digest) (*ocispec.Descriptor, error) {
	targetInfo, err := cs.Info(ctx, targetDigest)
	if err != nil {
		return nil, errors.Wrapf(err, "get target blob info %s", targetDigest)
	}
	if targetInfo.Labels == nil {
		targetInfo.Labels = map[string]string{}
	}
	// Write a diff id label of layer in content store for simplifying
	// diff id calculation to speed up the conversion.
	// See: https://github.com/containerd/containerd/blob/e4fefea5544d259177abb85b64e428702ac49c97/images/diffid.go#L49
	targetInfo.Labels[labels.LabelUncompressed] = targetDigest.String()
	_, err = cs.Update(ctx, targetInfo)
	if err != nil {
		return nil, errors.Wrap(err, "update layer label")
	}

	targetDesc := ocispec.Descriptor{
		Digest:    targetDigest,
		Size:      targetInfo.Size,
		MediaType: MediaTypeNydusBlob,
		Annotations: map[string]string{
			// Use `containerd.io/uncompressed` to generate DiffID of
			// layer defined in OCI spec.
			LayerAnnotationUncompressed: targetDigest.String(),
			LayerAnnotationNydusBlob:    "true",
		},
	}

	if opt.OCIRef {
		targetDesc.Annotations[label.NydusRefLayer] = sourceDigest.String()
	}

	if opt.Encrypt {
		targetDesc.Annotations[LayerAnnotationNydusEncryptedBlob] = "true"
	}

	return &targetDesc, nil
}

// LayerConvertFunc returns a function which converts an OCI image layer to
// a nydus blob layer, and set the media type to "application/vnd.oci.image.layer.nydus.blob.v1".
func LayerConvertFunc(opt PackOption) converter.ConvertFunc {
	return func(ctx context.Context, cs content.Store, desc ocispec.Descriptor) (*ocispec.Descriptor, error) {
		if ctx.Err() != nil {
			// The context is already cancelled, no need to proceed.
			return nil, ctx.Err()
		}
		if !images.IsLayerType(desc.MediaType) {
			return nil, nil
		}

		// Skip the conversion of nydus layer.
		if IsNydusBlob(desc) || IsNydusBootstrap(desc) {
			return nil, nil
		}

		// Use remote cache to avoid unnecessary conversion
		info, err := cs.Info(ctx, desc.Digest)
		if err != nil {
			return nil, errors.Wrapf(err, "get blob info %s", desc.Digest)
		}
		if targetDigest := digest.Digest(info.Labels[LayerAnnotationNydusTargetDigest]); targetDigest.Validate() == nil {
			return makeBlobDesc(ctx, cs, opt, desc.Digest, targetDigest)
		}

		ra, err := cs.ReaderAt(ctx, desc)
		if err != nil {
			return nil, errors.Wrap(err, "get source blob reader")
		}
		defer ra.Close()
		rdr := io.NewSectionReader(ra, 0, ra.Size())

		ref := fmt.Sprintf("convert-nydus-from-%s", desc.Digest)
		dst, err := content.OpenWriter(ctx, cs, content.WithRef(ref))
		if err != nil {
			return nil, errors.Wrap(err, "open blob writer")
		}
		defer dst.Close()

		var tr io.ReadCloser
		if opt.OCIRef {
			tr = io.NopCloser(rdr)
		} else {
			tr, err = compression.DecompressStream(rdr)
			if err != nil {
				return nil, errors.Wrap(err, "decompress blob stream")
			}
		}

		digester := digest.SHA256.Digester()
		pr, pw := io.Pipe()
		tw, err := Pack(ctx, io.MultiWriter(pw, digester.Hash()), opt)
		if err != nil {
			return nil, errors.Wrap(err, "pack tar to nydus")
		}

		copyBufferDone := make(chan error, 1)
		go func() {
			buffer := bufPool.Get().(*[]byte)
			defer bufPool.Put(buffer)
			_, err := io.CopyBuffer(tw, tr, *buffer)
			copyBufferDone <- err
		}()

		go func() {
			defer pw.Close()
			select {
			case <-ctx.Done():
				// The context was cancelled!
				// Close the pipe with the context's error to signal
				// the reader to stop.
				pw.CloseWithError(ctx.Err())
				return
			case err := <-copyBufferDone:
				if err != nil {
					pw.CloseWithError(err)
					return
				}
			}
			if err := tr.Close(); err != nil {
				pw.CloseWithError(err)
				return
			}
			if err := tw.Close(); err != nil {
				pw.CloseWithError(err)
				return
			}
		}()

		if err := content.Copy(ctx, dst, pr, 0, ""); err != nil {
			return nil, errors.Wrap(err, "copy nydus blob to content store")
		}

		blobDigest := digester.Digest()
		newDesc, err := makeBlobDesc(ctx, cs, opt, desc.Digest, blobDigest)
		if err != nil {
			return nil, err
		}

		if opt.Backend != nil {
			if err := opt.Backend.Push(ctx, cs, *newDesc); err != nil {
				return nil, errors.Wrap(err, "push to storage backend")
			}
		}

		return newDesc, nil
	}
}

// ConvertHookFunc returns a function which will be used as a callback
// called for each blob after conversion is done. The function only hooks
// the index conversion and the manifest conversion.
func ConvertHookFunc(opt MergeOption) converter.ConvertHookFunc {
	return func(ctx context.Context, cs content.Store, orgDesc ocispec.Descriptor, newDesc *ocispec.Descriptor) (*ocispec.Descriptor, error) {
		// If the previous conversion did not occur, the `newDesc` may be nil.
		if newDesc == nil {
			return &orgDesc, nil
		}
		switch {
		case images.IsIndexType(newDesc.MediaType):
			return convertIndex(ctx, cs, newDesc)
		case images.IsManifestType(newDesc.MediaType):
			return convertManifest(ctx, cs, orgDesc, newDesc, opt)
		default:
			return newDesc, nil
		}
	}
}

// convertIndex modifies the original index converting it to manifest directly if it contains only one manifest.
func convertIndex(ctx context.Context, cs content.Store, newDesc *ocispec.Descriptor) (*ocispec.Descriptor, error) {
	var index ocispec.Index
	_, err := readJSON(ctx, cs, &index, *newDesc)
	if err != nil {
		return nil, errors.Wrap(err, "read index json")
	}

	// If the converted manifest list contains only one manifest,
	// convert it directly to manifest.
	if len(index.Manifests) == 1 {
		return &index.Manifests[0], nil
	}
	return newDesc, nil
}

// convertManifest merges all the nydus blob layers into a
// nydus bootstrap layer, update the image config,
// and modify the image manifest.
func convertManifest(ctx context.Context, cs content.Store, oldDesc ocispec.Descriptor, newDesc *ocispec.Descriptor, opt MergeOption) (*ocispec.Descriptor, error) {
	var manifest ocispec.Manifest
	manifestDesc := *newDesc
	manifestLabels, err := readJSON(ctx, cs, &manifest, manifestDesc)
	if err != nil {
		return nil, errors.Wrap(err, "read manifest json")
	}

	if isNydusImage(&manifest) {
		return &manifestDesc, nil
	}

	// This option needs to be enabled for image scenario.
	opt.WithTar = true

	// If the original image is already an OCI type, we should forcibly set the
	// bootstrap layer to the OCI type.
	if !opt.OCI && oldDesc.MediaType == ocispec.MediaTypeImageManifest {
		opt.OCI = true
	}

	// Append bootstrap layer to manifest, encrypt bootstrap layer if needed.
	bootstrapDesc, blobDescs, err := MergeLayers(ctx, cs, manifest.Layers, opt)
	if err != nil {
		return nil, errors.Wrap(err, "merge nydus layers")
	}
	if opt.Backend != nil {
		// Only append nydus bootstrap layer into manifest, and do not put nydus
		// blob layer into manifest if blob storage backend is specified.
		manifest.Layers = []ocispec.Descriptor{*bootstrapDesc}
	} else {
		for idx, blobDesc := range blobDescs {
			blobGCLabelKey := fmt.Sprintf("containerd.io/gc.ref.content.l.%d", idx)
			manifestLabels[blobGCLabelKey] = blobDesc.Digest.String()
		}
		// Affected by chunk dict, the blob list referenced by final bootstrap
		// are from different layers, part of them are from original layers, part
		// from chunk dict bootstrap, so we need to rewrite manifest's layers here.
		blobDescs := append(blobDescs, *bootstrapDesc)
		manifest.Layers = blobDescs
	}

	// Update the gc label of bootstrap layer
	bootstrapGCLabelKey := fmt.Sprintf("containerd.io/gc.ref.content.l.%d", len(manifest.Layers)-1)
	manifestLabels[bootstrapGCLabelKey] = bootstrapDesc.Digest.String()

	// Rewrite diff ids and remove useless annotation.
	var config ocispec.Image
	configLabels, err := readJSON(ctx, cs, &config, manifest.Config)
	if err != nil {
		return nil, errors.Wrap(err, "read image config")
	}
	bootstrapHistory := ocispec.History{
		CreatedBy: "Nydus Converter",
		Comment:   "Nydus Bootstrap Layer",
	}
	if opt.Backend != nil {
		config.RootFS.DiffIDs = []digest.Digest{digest.Digest(bootstrapDesc.Annotations[LayerAnnotationUncompressed])}
		config.History = []ocispec.History{bootstrapHistory}
	} else {
		config.RootFS.DiffIDs = make([]digest.Digest, 0, len(manifest.Layers))
		for i, layer := range manifest.Layers {
			config.RootFS.DiffIDs = append(config.RootFS.DiffIDs, digest.Digest(layer.Annotations[LayerAnnotationUncompressed]))
			// Remove useless annotation.
			delete(manifest.Layers[i].Annotations, LayerAnnotationUncompressed)
		}
		// Append history item for bootstrap layer, to ensure the history consistency.
		// See https://github.com/distribution/distribution/blob/e5d5810851d1f17a5070e9b6f940d8af98ea3c29/manifest/schema1/config_builder.go#L136
		config.History = append(config.History, bootstrapHistory)
	}
	// Update image config in content store.
	newConfigDesc, err := writeJSON(ctx, cs, config, manifest.Config, configLabels)
	if err != nil {
		return nil, errors.Wrap(err, "write image config")
	}
	// When manifests are merged, we need to put a special value for the config mediaType.
	// This values must be one that containerd doesn't understand to ensure it doesn't try tu pull the nydus image
	// but use the OCI one instead. And then if the nydus-snapshotter is used, it can pull the nydus image instead.
	if opt.MergeManifest {
		newConfigDesc.MediaType = ManifestConfigNydus
	}
	manifest.Config = *newConfigDesc
	// Update the config gc label
	manifestLabels[configGCLabelKey] = newConfigDesc.Digest.String()

	if opt.WithReferrer {
		// Associate a reference to the original OCI manifest.
		// See the `subject` field description in
		// https://github.com/opencontainers/image-spec/blob/main/manifest.md#image-manifest-property-descriptions
		manifest.Subject = &oldDesc
		// Remove the platform field as it is not supported by certain registries like ECR.
		manifest.Subject.Platform = nil
	}

	// Update image manifest in content store.
	newManifestDesc, err := writeJSON(ctx, cs, manifest, manifestDesc, manifestLabels)
	if err != nil {
		return nil, errors.Wrap(err, "write manifest")
	}

	return newManifestDesc, nil
}

// MergeLayers merges a list of nydus blob layer into a nydus bootstrap layer.
// The media type of the nydus bootstrap layer is "application/vnd.oci.image.layer.v1.tar+gzip".
func MergeLayers(ctx context.Context, cs content.Store, descs []ocispec.Descriptor, opt MergeOption) (*ocispec.Descriptor, []ocispec.Descriptor, error) {
	// Extracts nydus bootstrap from nydus format for each layer.
	layers := []Layer{}

	var chainID digest.Digest
	nydusBlobDigests := []digest.Digest{}
	for _, nydusBlobDesc := range descs {
		ra, err := cs.ReaderAt(ctx, nydusBlobDesc)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "get reader for blob %q", nydusBlobDesc.Digest)
		}
		defer ra.Close()
		var originalDigest *digest.Digest
		if opt.OCIRef {
			digestStr := nydusBlobDesc.Annotations[label.NydusRefLayer]
			_originalDigest, err := digest.Parse(digestStr)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "invalid label %s=%s", label.NydusRefLayer, digestStr)
			}
			originalDigest = &_originalDigest
		}
		layers = append(layers, Layer{
			Digest:         nydusBlobDesc.Digest,
			OriginalDigest: originalDigest,
			ReaderAt:       ra,
		})
		if chainID == "" {
			chainID = identity.ChainID([]digest.Digest{nydusBlobDesc.Digest})
		} else {
			chainID = identity.ChainID([]digest.Digest{chainID, nydusBlobDesc.Digest})
		}
		nydusBlobDigests = append(nydusBlobDigests, nydusBlobDesc.Digest)
	}

	// Merge all nydus bootstraps into a final nydus bootstrap.
	pr, pw := io.Pipe()
	originalBlobDigestChan := make(chan []digest.Digest, 1)
	go func() {
		defer pw.Close()
		originalBlobDigests, err := Merge(ctx, layers, pw, opt)
		if err != nil {
			pw.CloseWithError(errors.Wrapf(err, "merge nydus bootstrap"))
		}
		originalBlobDigestChan <- originalBlobDigests
	}()

	// Compress final nydus bootstrap to tar.gz and write into content store.
	cw, err := content.OpenWriter(ctx, cs, content.WithRef("nydus-merge-"+chainID.String()))
	if err != nil {
		return nil, nil, errors.Wrap(err, "open content store writer")
	}
	defer cw.Close()

	gw := gzip.NewWriter(cw)
	uncompressedDgst := digest.SHA256.Digester()
	compressed := io.MultiWriter(gw, uncompressedDgst.Hash())
	buffer := bufPool.Get().(*[]byte)
	defer bufPool.Put(buffer)
	if _, err := io.CopyBuffer(compressed, pr, *buffer); err != nil {
		return nil, nil, errors.Wrapf(err, "copy bootstrap targz into content store")
	}
	if err := gw.Close(); err != nil {
		return nil, nil, errors.Wrap(err, "close gzip writer")
	}

	compressedDgst := cw.Digest()
	if err := cw.Commit(ctx, 0, compressedDgst, content.WithLabels(map[string]string{
		LayerAnnotationUncompressed: uncompressedDgst.Digest().String(),
	})); err != nil {
		if !errdefs.IsAlreadyExists(err) {
			return nil, nil, errors.Wrap(err, "commit to content store")
		}
	}
	if err := cw.Close(); err != nil {
		return nil, nil, errors.Wrap(err, "close content store writer")
	}

	bootstrapInfo, err := cs.Info(ctx, compressedDgst)
	if err != nil {
		return nil, nil, errors.Wrap(err, "get info from content store")
	}

	originalBlobDigests := <-originalBlobDigestChan
	blobDescs := []ocispec.Descriptor{}

	var blobDigests []digest.Digest
	if opt.OCIRef {
		blobDigests = nydusBlobDigests
	} else {
		blobDigests = originalBlobDigests
	}

	for idx, blobDigest := range blobDigests {
		blobInfo, err := cs.Info(ctx, blobDigest)
		if err != nil {
			return nil, nil, errors.Wrap(err, "get info from content store")
		}
		blobDesc := ocispec.Descriptor{
			Digest:    blobDigest,
			Size:      blobInfo.Size,
			MediaType: MediaTypeNydusBlob,
			Annotations: map[string]string{
				LayerAnnotationUncompressed: blobDigest.String(),
				LayerAnnotationNydusBlob:    "true",
			},
		}
		if opt.OCIRef {
			blobDesc.Annotations[label.NydusRefLayer] = layers[idx].OriginalDigest.String()
		}

		if opt.Encrypt != nil {
			blobDesc.Annotations[LayerAnnotationNydusEncryptedBlob] = "true"
		}

		blobDescs = append(blobDescs, blobDesc)
	}

	if opt.FsVersion == "" {
		opt.FsVersion = "6"
	}
	mediaType := images.MediaTypeDockerSchema2LayerGzip
	if opt.OCI {
		mediaType = ocispec.MediaTypeImageLayerGzip
	}

	bootstrapDesc := ocispec.Descriptor{
		Digest:    compressedDgst,
		Size:      bootstrapInfo.Size,
		MediaType: mediaType,
		Annotations: map[string]string{
			LayerAnnotationUncompressed: uncompressedDgst.Digest().String(),
			LayerAnnotationFSVersion:    opt.FsVersion,
			// Use this annotation to identify nydus bootstrap layer.
			LayerAnnotationNydusBootstrap: "true",
		},
	}

	if opt.Encrypt != nil {
		// Encrypt the Nydus bootstrap layer.
		bootstrapDesc, err = opt.Encrypt(ctx, cs, bootstrapDesc)
		if err != nil {
			return nil, nil, errors.Wrap(err, "encrypt bootstrap layer")
		}
	}
	return &bootstrapDesc, blobDescs, nil
}
