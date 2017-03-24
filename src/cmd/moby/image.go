package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
)

// This uses Docker to convert a Docker image into a tarball. It would be an improvement if we
// used the containerd libraries to do this instead locally direct from a local image
// cache as it would be much simpler.

var exclude = map[string]bool{
	".dockerenv":  true,
	"Dockerfile":  true,
	"dev/console": true,
	"dev/pts":     true,
	"dev/shm":     true,
}

var replace = map[string]string{
	"etc/hosts": `127.0.0.1       localhost
::1     localhost ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
`,
	"etc/resolv.conf": `nameserver 8.8.8.8
nameserver 8.8.4.4
nameserver 2001:4860:4860::8888
nameserver 2001:4860:4860::8844
`,
	"etc/hostname": "moby",
}

// ImageExtract extracts the filesystem from an image and returns a tarball with the files prefixed by the given path
func ImageExtract(image, prefix string) ([]byte, error) {
	out := new(bytes.Buffer)
	tw := tar.NewWriter(out)
	err := tarPrefix(prefix, tw)
	if err != nil {
		return []byte{}, err
	}
	err = imageTar(image, prefix, tw)
	if err != nil {
		return []byte{}, err
	}
	err = tw.Close()
	if err != nil {
		return []byte{}, err
	}
	return out.Bytes(), nil
}

// tarPrefix creates the leading directories for a path
func tarPrefix(path string, tw *tar.Writer) error {
	if path == "" {
		return nil
	}
	if path[len(path)-1] != byte('/') {
		return fmt.Errorf("path does not end with /: %s", path)
	}
	path = path[:len(path)-1]
	if path[0] == byte('/') {
		return fmt.Errorf("path should be relative: %s", path)
	}
	mkdir := ""
	for _, dir := range strings.Split(path, "/") {
		mkdir = mkdir + dir
		hdr := &tar.Header{
			Name:     mkdir,
			Mode:     0755,
			Typeflag: tar.TypeDir,
		}
		tw.WriteHeader(hdr)
		mkdir = mkdir + "/"
	}
	return nil
}

func imageTar(image, prefix string, tw *tar.Writer) error {
	if prefix != "" && prefix[len(prefix)-1] != byte('/') {
		return fmt.Errorf("prefix does not end with /: %s", prefix)
	}
	container, err := dockerCreate(image)
	if err != nil {
		return fmt.Errorf("Failed to docker create image %s: %v", image, err)
	}
	contents, err := dockerExport(container)
	if err != nil {
		return fmt.Errorf("Failed to docker export container from container %s: %v", container, err)
	}
	err = dockerRm(container)
	if err != nil {
		return fmt.Errorf("Failed to docker rm container %s: %v", container, err)
	}

	// now we need to filter out some files from the resulting tar archive

	r := bytes.NewReader(contents)
	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if exclude[hdr.Name] {
			io.Copy(ioutil.Discard, tr)
		} else if replace[hdr.Name] != "" {
			contents := replace[hdr.Name]
			hdr.Size = int64(len(contents))
			hdr.Name = prefix + hdr.Name
			tw.WriteHeader(hdr)
			buf := bytes.NewBufferString(contents)
			io.Copy(tw, buf)
			io.Copy(ioutil.Discard, tr)
		} else {
			hdr.Name = prefix + hdr.Name
			tw.WriteHeader(hdr)
			io.Copy(tw, tr)
		}
	}
	err = tw.Close()
	if err != nil {
		return err
	}
	return nil
}

// ImageBundle produces an OCI bundle at the given path in a tarball, given an image and a config.json
func ImageBundle(path, image, config string) ([]byte, error) {
	out := new(bytes.Buffer)
	tw := tar.NewWriter(out)
	err := tarPrefix(path+"/rootfs/", tw)
	if err != nil {
		return []byte{}, err
	}
	hdr := &tar.Header{
		Name: path + "/" + "config.json",
		Mode: 0644,
		Size: int64(len(config)),
	}
	err = tw.WriteHeader(hdr)
	if err != nil {
		return []byte{}, err
	}
	buf := bytes.NewBufferString(config)
	_, err = io.Copy(tw, buf)
	if err != nil {
		return []byte{}, err
	}
	err = imageTar(image, path+"/rootfs/", tw)
	if err != nil {
		return []byte{}, err
	}
	err = tw.Close()
	if err != nil {
		return []byte{}, err
	}
	return out.Bytes(), nil
}
