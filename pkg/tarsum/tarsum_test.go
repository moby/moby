package tarsum

import (
	"bytes"
	"crypto/rand"
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
}

var testLayers = []testLayer{
	{
		filename: "testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/layer.tar",
		jsonfile: "testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/json",
		tarsum:   "tarsum+sha256:e58fcf7418d4390dec8e8fb69d88c06ec07039d651fedd3aa72af9972e7d046b"},
	{
		filename: "testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/layer.tar",
		jsonfile: "testdata/46af0962ab5afeb5ce6740d4d91652e69206fc991fd5328c1a94d364ad00e457/json",
		gzip:     true,
		tarsum:   "tarsum+sha256:e58fcf7418d4390dec8e8fb69d88c06ec07039d651fedd3aa72af9972e7d046b"},
	{
		filename: "testdata/511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158/layer.tar",
		jsonfile: "testdata/511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158/json",
		tarsum:   "tarsum+sha256:ac672ee85da9ab7f9667ae3c32841d3e42f33cc52c273c23341dabba1c8b0c8b"},
	{
		options: &sizedOptions{1, 1024 * 1024, false, false}, // a 1mb file (in memory)
		tarsum:  "tarsum+sha256:8bf12d7e67c51ee2e8306cba569398b1b9f419969521a12ffb9d8875e8836738"},
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
		ts := &TarSum{Reader: fh, DisableCompression: !layer.gzip}
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
		ts := &TarSum{Reader: buf, DisableCompression: true}
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
		ts := &TarSum{Reader: buf, DisableCompression: false}
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
		ts := &TarSum{Reader: fh, DisableCompression: !isGzip}
		io.Copy(ioutil.Discard, ts)
		ts.Sum(nil)
		fh.Seek(0, 0)
	}
}
