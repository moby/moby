package docker

import (
	"bytes"
	"github.com/dotcloud/docker/vendor/src/code.google.com/p/go/src/pkg/archive/tar"
	"io"
)

func fakeTar() (io.Reader, error) {
	content := []byte("Hello world!\n")
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	for _, name := range []string{"/etc/postgres/postgres.conf", "/etc/passwd", "/var/log/postgres/postgres.conf"} {
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
