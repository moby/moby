package tarsum // import "github.com/docker/docker/pkg/tarsum"

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5" // #nosec G501
	"crypto/rand"
	"crypto/sha1" // #nosec G505
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type testLayer struct {
	filename string
	options  *sizedOptions
	jsonfile string
	gzip     bool
	tarsum   string
	version  Version
	hash     THash
}

var testLayers = []testLayer{
	{
		filename: "testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/layer.tar",
		jsonfile: "testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/json",
		version:  Version0,
		tarsum:   "tarsum+sha256:4095cc12fa5fdb1ab2760377e1cd0c4ecdd3e61b4f9b82319d96fcea6c9a41c6"},
	{
		filename: "testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/layer.tar",
		jsonfile: "testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/json",
		version:  VersionDev,
		tarsum:   "tarsum.dev+sha256:db56e35eec6ce65ba1588c20ba6b1ea23743b59e81fb6b7f358ccbde5580345c"},
	{
		filename: "testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/layer.tar",
		jsonfile: "testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/json",
		gzip:     true,
		tarsum:   "tarsum+sha256:4095cc12fa5fdb1ab2760377e1cd0c4ecdd3e61b4f9b82319d96fcea6c9a41c6"},
	{
		// Tests existing version of TarSum when xattrs are present
		filename: "testdata/xattr/layer.tar",
		jsonfile: "testdata/xattr/json",
		version:  Version0,
		tarsum:   "tarsum+sha256:07e304a8dbcb215b37649fde1a699f8aeea47e60815707f1cdf4d55d25ff6ab4"},
	{
		// Tests next version of TarSum when xattrs are present
		filename: "testdata/xattr/layer.tar",
		jsonfile: "testdata/xattr/json",
		version:  VersionDev,
		tarsum:   "tarsum.dev+sha256:6c58917892d77b3b357b0f9ad1e28e1f4ae4de3a8006bd3beb8beda214d8fd16"},
	{
		filename: "testdata/511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158/layer.tar",
		jsonfile: "testdata/511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158/json",
		tarsum:   "tarsum+sha256:c66bd5ec9f87b8f4c6135ca37684618f486a3dd1d113b138d0a177bfa39c2571"},
	{
		options: &sizedOptions{1, 1024 * 1024, false, false}, // a 1mb file (in memory)
		tarsum:  "tarsum+sha256:75258b2c5dcd9adfe24ce71eeca5fc5019c7e669912f15703ede92b1a60cb11f"},
	{
		// this tar has two files with the same path
		filename: "testdata/collision/collision-0.tar",
		tarsum:   "tarsum+sha256:7cabb5e9128bb4a93ff867b9464d7c66a644ae51ea2e90e6ef313f3bef93f077"},
	{
		// this tar has the same two files (with the same path), but reversed order. ensuring is has different hash than above
		filename: "testdata/collision/collision-1.tar",
		tarsum:   "tarsum+sha256:805fd393cfd58900b10c5636cf9bab48b2406d9b66523122f2352620c85dc7f9"},
	{
		// this tar has newer of collider-0.tar, ensuring is has different hash
		filename: "testdata/collision/collision-2.tar",
		tarsum:   "tarsum+sha256:85d2b8389f077659d78aca898f9e632ed9161f553f144aef100648eac540147b"},
	{
		// this tar has newer of collider-1.tar, ensuring is has different hash
		filename: "testdata/collision/collision-3.tar",
		tarsum:   "tarsum+sha256:cbe4dee79fe979d69c16c2bccd032e3205716a562f4a3c1ca1cbeed7b256eb19"},
	{
		options: &sizedOptions{1, 1024 * 1024, false, false}, // a 1mb file (in memory)
		tarsum:  "tarsum+md5:3a6cdb475d90459ac0d3280703d17be2",
		hash:    md5THash,
	},
	{
		options: &sizedOptions{1, 1024 * 1024, false, false}, // a 1mb file (in memory)
		tarsum:  "tarsum+sha1:14b5e0d12a0c50a4281e86e92153fa06d55d00c6",
		hash:    sha1Hash,
	},
	{
		options: &sizedOptions{1, 1024 * 1024, false, false}, // a 1mb file (in memory)
		tarsum:  "tarsum+sha224:dd8925b7a4c71b13f3a68a0f9428a757c76b93752c398f272a9062d5",
		hash:    sha224Hash,
	},
	{
		options: &sizedOptions{1, 1024 * 1024, false, false}, // a 1mb file (in memory)
		tarsum:  "tarsum+sha384:e39e82f40005134bed13fb632d1a5f2aa4675c9ddb4a136fbcec202797e68d2f635e1200dee2e3a8d7f69d54d3f2fd27",
		hash:    sha384Hash,
	},
	{
		options: &sizedOptions{1, 1024 * 1024, false, false}, // a 1mb file (in memory)
		tarsum:  "tarsum+sha512:7c56de40b2d1ed3863ff25d83b59cdc8f53e67d1c01c3ee8f201f8e4dec3107da976d0c0ec9109c962a152b32699fe329b2dab13966020e400c32878a0761a7e",
		hash:    sha512Hash,
	},
}

type sizedOptions struct {
	num      int64
	size     int64
	isRand   bool
	realFile bool
}

// make a tar:
// * num is the number of files the tar should have
// * size is the bytes per file
// * isRand is whether the contents of the files should be a random chunk (otherwise it's all zeros)
// * realFile will write to a TempFile, instead of an in memory buffer
func sizedTar(opts sizedOptions) io.Reader {
	var (
		fh  io.ReadWriter
		err error
	)
	if opts.realFile {
		fh, err = os.CreateTemp("", "tarsum")
		if err != nil {
			return nil
		}
	} else {
		fh = bytes.NewBuffer([]byte{})
	}
	tarW := tar.NewWriter(fh)
	defer tarW.Close()
	for i := int64(0); i < opts.num; i++ {
		err := tarW.WriteHeader(&tar.Header{
			Name:     fmt.Sprintf("/testdata%d", i),
			Mode:     0755,
			Uid:      0,
			Gid:      0,
			Size:     opts.size,
			Typeflag: tar.TypeReg,
		})
		if err != nil {
			return nil
		}
		var rBuf []byte
		if opts.isRand {
			rBuf = make([]byte, 8)
			_, err = rand.Read(rBuf)
			if err != nil {
				return nil
			}
		} else {
			rBuf = []byte{0, 0, 0, 0, 0, 0, 0, 0}
		}

		for i := int64(0); i < opts.size/int64(8); i++ {
			tarW.Write(rBuf)
		}
	}
	return fh
}

func emptyTarSum(gzip bool) (TarSum, error) {
	reader, writer := io.Pipe()
	tarWriter := tar.NewWriter(writer)

	// Immediately close tarWriter and write-end of the
	// Pipe in a separate goroutine so we don't block.
	go func() {
		tarWriter.Close()
		writer.Close()
	}()

	return NewTarSum(reader, !gzip, Version0)
}

// Test errors on NewTarsumForLabel
func TestNewTarSumForLabelInvalid(t *testing.T) {
	reader := strings.NewReader("")

	if _, err := NewTarSumForLabel(reader, true, "invalidlabel"); err == nil {
		t.Fatalf("Expected an error, got nothing.")
	}

	if _, err := NewTarSumForLabel(reader, true, "invalid+sha256"); err == nil {
		t.Fatalf("Expected an error, got nothing.")
	}
	if _, err := NewTarSumForLabel(reader, true, "tarsum.v1+invalid"); err == nil {
		t.Fatalf("Expected an error, got nothing.")
	}
}

func TestNewTarSumForLabel(t *testing.T) {

	layer := testLayers[0]

	reader, err := os.Open(layer.filename)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	label := strings.Split(layer.tarsum, ":")[0]
	ts, err := NewTarSumForLabel(reader, false, label)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure it actually worked by reading a little bit of it
	nbByteToRead := 8 * 1024
	dBuf := make([]byte, nbByteToRead)
	_, err = ts.Read(dBuf)
	if err != nil {
		t.Errorf("failed to read %vKB from %s: %s", nbByteToRead, layer.filename, err)
	}
}

// TestEmptyTar tests that tarsum does not fail to read an empty tar
// and correctly returns the hex digest of an empty hash.
func TestEmptyTar(t *testing.T) {
	// Test without gzip.
	ts, err := emptyTarSum(false)
	assert.NilError(t, err)

	zeroBlock := make([]byte, 1024)
	buf := new(bytes.Buffer)

	n, err := io.Copy(buf, ts)
	assert.NilError(t, err)

	if n != int64(len(zeroBlock)) || !bytes.Equal(buf.Bytes(), zeroBlock) {
		t.Fatalf("tarSum did not write the correct number of zeroed bytes: %d", n)
	}

	expectedSum := ts.Version().String() + "+sha256:" + hex.EncodeToString(sha256.New().Sum(nil))
	resultSum := ts.Sum(nil)

	if resultSum != expectedSum {
		t.Fatalf("expected [%s] but got [%s]", expectedSum, resultSum)
	}

	// Test with gzip.
	ts, err = emptyTarSum(true)
	assert.NilError(t, err)
	buf.Reset()

	_, err = io.Copy(buf, ts)
	assert.NilError(t, err)

	bufgz := new(bytes.Buffer)
	gz := gzip.NewWriter(bufgz)
	n, err = io.Copy(gz, bytes.NewBuffer(zeroBlock))
	assert.NilError(t, err)
	gz.Close()
	gzBytes := bufgz.Bytes()

	if n != int64(len(zeroBlock)) || !bytes.Equal(buf.Bytes(), gzBytes) {
		t.Fatalf("tarSum did not write the correct number of gzipped-zeroed bytes: %d", n)
	}

	resultSum = ts.Sum(nil)

	if resultSum != expectedSum {
		t.Fatalf("expected [%s] but got [%s]", expectedSum, resultSum)
	}

	// Test without ever actually writing anything.
	if ts, err = NewTarSum(bytes.NewReader([]byte{}), true, Version0); err != nil {
		t.Fatal(err)
	}

	resultSum = ts.Sum(nil)
	assert.Check(t, is.Equal(expectedSum, resultSum))
}

var (
	md5THash   = NewTHash("md5", md5.New)
	sha1Hash   = NewTHash("sha1", sha1.New)
	sha224Hash = NewTHash("sha224", sha256.New224)
	sha384Hash = NewTHash("sha384", sha512.New384)
	sha512Hash = NewTHash("sha512", sha512.New)
)

// Test all the build-in read size : buf8K, buf16K, buf32K and more
func TestTarSumsReadSize(t *testing.T) {
	// Test always on the same layer (that is big enough)
	layer := testLayers[0]

	for i := 0; i < 5; i++ {

		reader, err := os.Open(layer.filename)
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()

		ts, err := NewTarSum(reader, false, layer.version)
		if err != nil {
			t.Fatal(err)
		}

		// Read and discard bytes so that it populates sums
		nbByteToRead := (i + 1) * 8 * 1024
		dBuf := make([]byte, nbByteToRead)
		_, err = ts.Read(dBuf)
		if err != nil {
			t.Errorf("failed to read %vKB from %s: %s", nbByteToRead, layer.filename, err)
			continue
		}
	}
}

func TestTarSums(t *testing.T) {
	for _, layer := range testLayers {
		var (
			fh  io.Reader
			err error
		)
		if len(layer.filename) > 0 {
			fh, err = os.Open(layer.filename)
			if err != nil {
				t.Errorf("failed to open %s: %s", layer.filename, err)
				continue
			}
		} else if layer.options != nil {
			fh = sizedTar(*layer.options)
		} else {
			// What else is there to test?
			t.Errorf("what to do with %#v", layer)
			continue
		}
		if file, ok := fh.(*os.File); ok {
			defer file.Close()
		}

		var ts TarSum
		if layer.hash == nil {
			//                           double negatives!
			ts, err = NewTarSum(fh, !layer.gzip, layer.version)
		} else {
			ts, err = NewTarSumHash(fh, !layer.gzip, layer.version, layer.hash)
		}
		if err != nil {
			t.Errorf("%q :: %q", err, layer.filename)
			continue
		}

		// Read variable number of bytes to test dynamic buffer
		dBuf := make([]byte, 1)
		_, err = ts.Read(dBuf)
		if err != nil {
			t.Errorf("failed to read 1B from %s: %s", layer.filename, err)
			continue
		}
		dBuf = make([]byte, 16*1024)
		_, err = ts.Read(dBuf)
		if err != nil {
			t.Errorf("failed to read 16KB from %s: %s", layer.filename, err)
			continue
		}

		// Read and discard remaining bytes
		_, err = io.Copy(io.Discard, ts)
		if err != nil {
			t.Errorf("failed to copy from %s: %s", layer.filename, err)
			continue
		}
		var gotSum string
		if len(layer.jsonfile) > 0 {
			jfh, err := os.Open(layer.jsonfile)
			if err != nil {
				t.Errorf("failed to open %s: %s", layer.jsonfile, err)
				continue
			}
			defer jfh.Close()

			buf, err := io.ReadAll(jfh)
			if err != nil {
				t.Errorf("failed to readAll %s: %s", layer.jsonfile, err)
				continue
			}
			gotSum = ts.Sum(buf)
		} else {
			gotSum = ts.Sum(nil)
		}

		if layer.tarsum != gotSum {
			t.Errorf("expecting [%s], but got [%s]", layer.tarsum, gotSum)
		}
		var expectedHashName string
		if layer.hash != nil {
			expectedHashName = layer.hash.Name()
		} else {
			expectedHashName = DefaultTHash.Name()
		}
		if expectedHashName != ts.Hash().Name() {
			t.Errorf("expecting hash [%v], but got [%s]", expectedHashName, ts.Hash().Name())
		}
	}
}

func TestIteration(t *testing.T) {
	headerTests := []struct {
		expectedSum string // TODO(vbatts) it would be nice to get individual sums of each
		version     Version
		hdr         *tar.Header
		data        []byte
	}{
		{
			"tarsum+sha256:626c4a2e9a467d65c33ae81f7f3dedd4de8ccaee72af73223c4bc4718cbc7bbd",
			Version0,
			&tar.Header{
				Name:     "file.txt",
				Size:     0,
				Typeflag: tar.TypeReg,
				Devminor: 0,
				Devmajor: 0,
			},
			[]byte(""),
		},
		{
			"tarsum.dev+sha256:6ffd43a1573a9913325b4918e124ee982a99c0f3cba90fc032a65f5e20bdd465",
			VersionDev,
			&tar.Header{
				Name:     "file.txt",
				Size:     0,
				Typeflag: tar.TypeReg,
				Devminor: 0,
				Devmajor: 0,
			},
			[]byte(""),
		},
		{
			"tarsum.dev+sha256:862964db95e0fa7e42836ae4caab3576ab1df8d275720a45bdd01a5a3730cc63",
			VersionDev,
			&tar.Header{
				Name:     "another.txt",
				Uid:      1000,
				Gid:      1000,
				Uname:    "slartibartfast",
				Gname:    "users",
				Size:     4,
				Typeflag: tar.TypeReg,
				Devminor: 0,
				Devmajor: 0,
			},
			[]byte("test"),
		},
		{
			"tarsum.dev+sha256:4b1ba03544b49d96a32bacc77f8113220bd2f6a77e7e6d1e7b33cd87117d88e7",
			VersionDev,
			&tar.Header{
				Name:     "xattrs.txt",
				Uid:      1000,
				Gid:      1000,
				Uname:    "slartibartfast",
				Gname:    "users",
				Size:     4,
				Typeflag: tar.TypeReg,
				Xattrs: map[string]string{
					"user.key1": "value1",
					"user.key2": "value2",
				},
			},
			[]byte("test"),
		},
		{
			"tarsum.dev+sha256:410b602c898bd4e82e800050f89848fc2cf20fd52aa59c1ce29df76b878b84a6",
			VersionDev,
			&tar.Header{
				Name:     "xattrs.txt",
				Uid:      1000,
				Gid:      1000,
				Uname:    "slartibartfast",
				Gname:    "users",
				Size:     4,
				Typeflag: tar.TypeReg,
				Xattrs: map[string]string{
					"user.KEY1": "value1", // adding different case to ensure different sum
					"user.key2": "value2",
				},
			},
			[]byte("test"),
		},
		{
			"tarsum+sha256:b1f97eab73abd7593c245e51070f9fbdb1824c6b00a0b7a3d7f0015cd05e9e86",
			Version0,
			&tar.Header{
				Name:     "xattrs.txt",
				Uid:      1000,
				Gid:      1000,
				Uname:    "slartibartfast",
				Gname:    "users",
				Size:     4,
				Typeflag: tar.TypeReg,
				Xattrs: map[string]string{
					"user.NOT": "CALCULATED",
				},
			},
			[]byte("test"),
		},
	}
	for _, htest := range headerTests {
		s, err := renderSumForHeader(htest.version, htest.hdr, htest.data)
		if err != nil {
			t.Fatal(err)
		}

		if s != htest.expectedSum {
			t.Errorf("expected sum: %q, got: %q", htest.expectedSum, s)
		}
	}

}

func renderSumForHeader(v Version, h *tar.Header, data []byte) (string, error) {
	buf := bytes.NewBuffer(nil)
	// first build our test tar
	tw := tar.NewWriter(buf)
	if err := tw.WriteHeader(h); err != nil {
		return "", err
	}
	if _, err := tw.Write(data); err != nil {
		return "", err
	}
	tw.Close()

	ts, err := NewTarSum(buf, true, v)
	if err != nil {
		return "", err
	}
	tr := tar.NewReader(ts)
	for {
		hdr, err := tr.Next()
		if hdr == nil || err == io.EOF {
			// Signals the end of the archive.
			break
		}
		if err != nil {
			return "", err
		}
		if _, err = io.Copy(io.Discard, tr); err != nil {
			return "", err
		}
	}
	return ts.Sum(nil), nil
}

func Benchmark9kTar(b *testing.B) {
	buf := bytes.NewBuffer([]byte{})
	fh, err := os.Open("testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/layer.tar")
	if err != nil {
		b.Error(err)
		return
	}
	defer fh.Close()

	n, err := io.Copy(buf, fh)
	if err != nil {
		b.Error(err)
		return
	}

	reader := bytes.NewReader(buf.Bytes())

	b.SetBytes(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Seek(0, 0)
		ts, err := NewTarSum(reader, true, Version0)
		if err != nil {
			b.Error(err)
			return
		}
		io.Copy(io.Discard, ts)
		ts.Sum(nil)
	}
}

func Benchmark9kTarGzip(b *testing.B) {
	buf := bytes.NewBuffer([]byte{})
	fh, err := os.Open("testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/layer.tar")
	if err != nil {
		b.Error(err)
		return
	}
	defer fh.Close()

	n, err := io.Copy(buf, fh)
	if err != nil {
		b.Error(err)
		return
	}

	reader := bytes.NewReader(buf.Bytes())

	b.SetBytes(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Seek(0, 0)
		ts, err := NewTarSum(reader, false, Version0)
		if err != nil {
			b.Error(err)
			return
		}
		io.Copy(io.Discard, ts)
		ts.Sum(nil)
	}
}

// this is a single big file in the tar archive
func Benchmark1mbSingleFileTar(b *testing.B) {
	benchmarkTar(b, sizedOptions{1, 1024 * 1024, true, true}, false)
}

// this is a single big file in the tar archive
func Benchmark1mbSingleFileTarGzip(b *testing.B) {
	benchmarkTar(b, sizedOptions{1, 1024 * 1024, true, true}, true)
}

// this is 1024 1k files in the tar archive
func Benchmark1kFilesTar(b *testing.B) {
	benchmarkTar(b, sizedOptions{1024, 1024, true, true}, false)
}

// this is 1024 1k files in the tar archive
func Benchmark1kFilesTarGzip(b *testing.B) {
	benchmarkTar(b, sizedOptions{1024, 1024, true, true}, true)
}

func benchmarkTar(b *testing.B, opts sizedOptions, isGzip bool) {
	var fh *os.File
	tarReader := sizedTar(opts)
	if br, ok := tarReader.(*os.File); ok {
		fh = br
	}
	defer os.Remove(fh.Name())
	defer fh.Close()

	b.SetBytes(opts.size * opts.num)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts, err := NewTarSum(fh, !isGzip, Version0)
		if err != nil {
			b.Error(err)
			return
		}
		io.Copy(io.Discard, ts)
		ts.Sum(nil)
		fh.Seek(0, 0)
	}
}
