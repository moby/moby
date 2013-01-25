package fake

import (
	"github.com/dotcloud/docker/future"
	"bytes"
	"math/rand"
	"time"
	"io"
	"archive/tar"
	"fmt"
)

func Seed() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func randomBytes() io.Reader {
	return bytes.NewBuffer([]byte(fmt.Sprintf("%x", rand.Int())))
}

func FakeTar() (io.Reader, error) {
	content := []byte("Hello world!\n")
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	for _, name := range []string {"/etc/postgres/postgres.conf", "/etc/passwd", "/var/log/postgres", "/var/log/postgres/postgres.conf"} {
		hdr := new(tar.Header)
		hdr.Size = int64(len(content))
		hdr.Name = name
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		tw.Write([]byte(content))
	}
	tw.Close()
	return buf, nil
}


func WriteFakeTar(dst io.Writer) error {
	if data, err := FakeTar(); err != nil {
		return err
	} else if _, err := io.Copy(dst, data); err != nil {
		return err
	}
	return nil
}


func RandomId() string {
	id, _ := future.ComputeId(randomBytes()) // can't fail
	return id
}


func RandomBytesChanged() uint {
	return uint(rand.Int31n(24 * 1024 * 1024))
}

func RandomFilesChanged() uint {
	return uint(rand.Int31n(42))
}

func RandomContainerSize() uint {
	return uint(rand.Int31n(142 * 1024 * 1024))
}
