package tarsum

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/vendor/src/code.google.com/p/go/src/pkg/archive/tar"
)

type testLayer struct {
	filename string
	options  *sizedOptions
	jsonfile string
	gzip     bool
	tarsum   string
	version  Version
}

var testLayers = []testLayer{
	{
		filename: "testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/layer.tar",
		jsonfile: "testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/json",
		version:  Version0,
		tarsum:   "tarsum+sha256:e58fcf7418d4390dec8e8fb69d88c06ec07039d651fedd3aa72af9972e7d046b"},
	{
		filename: "testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/layer.tar",
		jsonfile: "testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/json",
		version:  VersionDev,
		tarsum:   "tarsum.dev+sha256:486b86e25c4db4551228154848bc4663b15dd95784b1588980f4ba1cb42e83e9"},
	{
		filename: "testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/layer.tar",
		jsonfile: "testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/json",
		gzip:     true,
		tarsum:   "tarsum+sha256:e58fcf7418d4390dec8e8fb69d88c06ec07039d651fedd3aa72af9972e7d046b"},
	{
		// Tests existing version of TarSum when xattrs are present
		filename: "testdata/xattr/layer.tar",
		jsonfile: "testdata/xattr/json",
		version:  Version0,
		tarsum:   "tarsum+sha256:e86f81a4d552f13039b1396ed03ca968ea9717581f9577ef1876ea6ff9b38c98"},
	{
		// Tests next version of TarSum when xattrs are present
		filename: "testdata/xattr/layer.tar",
		jsonfile: "testdata/xattr/json",
		version:  VersionDev,
		tarsum:   "tarsum.dev+sha256:6235cd3a2afb7501bac541772a3d61a3634e95bc90bb39a4676e2cb98d08390d"},
	{
		filename: "testdata/511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158/layer.tar",
		jsonfile: "testdata/511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158/json",
		tarsum:   "tarsum+sha256:ac672ee85da9ab7f9667ae3c32841d3e42f33cc52c273c23341dabba1c8b0c8b"},
	{
		options: &sizedOptions{1, 1024 * 1024, false, false}, // a 1mb file (in memory)
		tarsum:  "tarsum+sha256:8bf12d7e67c51ee2e8306cba569398b1b9f419969521a12ffb9d8875e8836738"},
	{
		// this tar has two files with the same path
		filename: "testdata/collision/collision-0.tar",
		tarsum:   "tarsum+sha256:08653904a68d3ab5c59e65ef58c49c1581caa3c34744f8d354b3f575ea04424a"},
	{
		// this tar has the same two files (with the same path), but reversed order. ensuring is has different hash than above
		filename: "testdata/collision/collision-1.tar",
		tarsum:   "tarsum+sha256:b51c13fbefe158b5ce420d2b930eef54c5cd55c50a2ee4abdddea8fa9f081e0d"},
	{
		// this tar has newer of collider-0.tar, ensuring is has different hash
		filename: "testdata/collision/collision-2.tar",
		tarsum:   "tarsum+sha256:381547080919bb82691e995508ae20ed33ce0f6948d41cafbeb70ce20c73ee8e"},
	{
		// this tar has newer of collider-1.tar, ensuring is has different hash
		filename: "testdata/collision/collision-3.tar",
		tarsum:   "tarsum+sha256:f886e431c08143164a676805205979cd8fa535dfcef714db5515650eea5a7c0f"},
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
		fh, err = ioutil.TempFile("", "tarsum")
		if err != nil {
			return nil
		}
	} else {
		fh = bytes.NewBuffer([]byte{})
	}
	tarW := tar.NewWriter(fh)
	for i := int64(0); i < opts.num; i++ {
		err := tarW.WriteHeader(&tar.Header{
			Name: fmt.Sprintf("/testdata%d", i),
			Mode: 0755,
			Uid:  0,
			Gid:  0,
			Size: opts.size,
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

// TestEmptyTar tests that tarsum does not fail to read an empty tar
// and correctly returns the hex digest of an empty hash.
func TestEmptyTar(t *testing.T) {
	// Test without gzip.
	ts, err := emptyTarSum(false)
	if err != nil {
		t.Fatal(err)
	}

	zeroBlock := make([]byte, 1024)
	buf := new(bytes.Buffer)

	n, err := io.Copy(buf, ts)
	if err != nil {
		t.Fatal(err)
	}

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
	if err != nil {
		t.Fatal(err)
	}
	buf.Reset()

	n, err = io.Copy(buf, ts)
	if err != nil {
		t.Fatal(err)
	}

	bufgz := new(bytes.Buffer)
	gz := gzip.NewWriter(bufgz)
	n, err = io.Copy(gz, bytes.NewBuffer(zeroBlock))
	gz.Close()
	gzBytes := bufgz.Bytes()

	if n != int64(len(zeroBlock)) || !bytes.Equal(buf.Bytes(), gzBytes) {
		t.Fatalf("tarSum did not write the correct number of gzipped-zeroed bytes: %d", n)
	}

	resultSum = ts.Sum(nil)

	if resultSum != expectedSum {
		t.Fatalf("expected [%s] but got [%s]", expectedSum, resultSum)
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

		//                                  double negatives!
		ts, err := NewTarSum(fh, !layer.gzip, layer.version)
		if err != nil {
			t.Errorf("%q :: %q", err, layer.filename)
			continue
		}
		_, err = io.Copy(ioutil.Discard, ts)
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
			buf, err := ioutil.ReadAll(jfh)
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
	}
}

func Benchmark9kTar(b *testing.B) {
	buf := bytes.NewBuffer([]byte{})
	fh, err := os.Open("testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/layer.tar")
	if err != nil {
		b.Error(err)
		return
	}
	n, err := io.Copy(buf, fh)
	fh.Close()

	b.SetBytes(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts, err := NewTarSum(buf, true, Version0)
		if err != nil {
			b.Error(err)
			return
		}
		io.Copy(ioutil.Discard, ts)
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
	n, err := io.Copy(buf, fh)
	fh.Close()

	b.SetBytes(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts, err := NewTarSum(buf, false, Version0)
		if err != nil {
			b.Error(err)
			return
		}
		io.Copy(ioutil.Discard, ts)
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
		io.Copy(ioutil.Discard, ts)
		ts.Sum(nil)
		fh.Seek(0, 0)
	}
}
