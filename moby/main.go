package main

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"

	"github.com/docker/moby/pkg/initrd"
)

const (
	docker2tar = "mobylinux/docker2tar:82a3f11f70b2959c7100dd6e184b511ebfc65908@sha256:e4fd36febc108477a2e5316d263ac257527779409891c7ac10d455a162df05c1"
)

func untarKernel(buf *bytes.Buffer, bzimageName, ktarName string) (*bytes.Buffer, *bytes.Buffer, error) {
	tr := tar.NewReader(buf)

	var bzimage, ktar *bytes.Buffer

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalln(err)
		}
		switch hdr.Name {
		case bzimageName:
			bzimage = new(bytes.Buffer)
			_, err := io.Copy(bzimage, tr)
			if err != nil {
				return nil, nil, err
			}
		case ktarName:
			ktar = new(bytes.Buffer)
			_, err := io.Copy(bzimage, tr)
			if err != nil {
				return nil, nil, err
			}
		default:
			continue
		}
	}

	if ktar == nil || bzimage == nil {
		return nil, nil, errors.New("did not find bzImage and kernel.tar in tarball")
	}

	return bzimage, ktar, nil
}

func containersInitrd(containers []*bytes.Buffer) (*bytes.Buffer, error) {
	w := new(bytes.Buffer)
	iw := initrd.NewWriter(w)
	defer iw.Close()
	for _, file := range containers {
		_, err := initrd.Copy(iw, file)
		if err != nil {
			return nil, err
		}
	}

	return w, nil
}

func build(configfile string) {
	config, err := ioutil.ReadFile(configfile)
	if err != nil {
		log.Fatalf("Cannot open config file: %v", err)
	}

	m, err := NewConfig(config)
	if err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	// TODO switch to using Docker client API not exec - just a quick prototype

	docker, err := exec.LookPath("docker")
	if err != nil {
		log.Fatalf("Docker does not seem to be installed")
	}

	containers := []*bytes.Buffer{}

	// get kernel bzImage and initrd tarball from container
	// TODO examine contents to see what names they might have
	const (
		bzimageName = "bzImage"
		ktarName    = "kernel.tar"
	)
	args := []string{"run", "--rm", m.Kernel, "tar", "cf", "-", bzimageName, ktarName}
	cmd := exec.Command(docker, args...)
	out, err := cmd.Output()
	if err != nil {
		log.Fatalf("Failed to extract kernel image and tarball")
	}
	buf := bytes.NewBuffer(out)
	bzimage, ktar, err := untarKernel(buf, bzimageName, ktarName)
	if err != nil {
		log.Fatalf("Could not extract bzImage and kernel filesystem from tarball")
	}
	containers = append(containers, ktar)

	// convert init image to tarball
	args = []string{"run", "--rm", "-v", "/var/run/docker.sock:/var/run/docker.sock", docker2tar, m.Init}
	cmd = exec.Command(docker, args...)
	init, err := cmd.Output()
	if err != nil {
		log.Fatalf("Failed to build init tarball: %v", err)
	}
	buffer := bytes.NewBuffer(init)
	containers = append(containers, buffer)

	for _, image := range m.System {
		args := ConfigToRun(&image)
		cmd := exec.Command(docker, args...)

		// get output tarball
		out, err := cmd.Output()
		if err != nil {
			log.Fatalf("Failed to build container tarball: %v", err)
		}
		buffer := bytes.NewBuffer(out)
		containers = append(containers, buffer)
	}

	// add files
	buffer, err = Filesystem(m)
	if err != nil {
		log.Fatalf("failed to add filesystem parts: %v", err)
	}
	containers = append(containers, buffer)

	initrd, err := containersInitrd(containers)
	if err != nil {
		log.Fatalf("Failed to make initrd %v", err)
	}

	for _, o := range m.Outputs {
		switch o.Format {
		case "kernel+initrd":
			err = OutputKernelInitrd(bzimage.Bytes(), initrd.Bytes())
			if err != nil {
				log.Fatalf("Error writing %s output: %v", o.Format, err)
			}
		case "":
			log.Fatalf("No format specified for output")
		default:
			log.Fatalf("Unknown output type %s", o.Format)
		}
	}
}

func OutputKernelInitrd(bzimage []byte, initrd []byte) error {
	err := ioutil.WriteFile("initrd.img", initrd, os.FileMode(0644))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile("bzImage", bzimage, os.FileMode(0644))
	if err != nil {
		return err
	}
	return nil
}

func main() {
	build("moby.yaml")
}
