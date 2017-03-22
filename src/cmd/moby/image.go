package main

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"
)

// This uses Docker to convert a Docker image into a tarball. It would be an improvement if we
// used the containerd libraries to do this instead locally direct from a local image
// cache as it would be much simpler.

func dockerCreate(image string) (string, error) {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return "", errors.New("Docker does not seem to be installed")
	}
	// we do not ever run the container, so /dev/null is used as command
	args := []string{"create", image, "/dev/null"}
	cmd := exec.Command(docker, args...)

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	err = cmd.Start()
	if err != nil {
		return "", err
	}

	stdout, err := ioutil.ReadAll(stdoutPipe)
	if err != nil {
		return "", err
	}

	stderr, err := ioutil.ReadAll(stderrPipe)
	if err != nil {
		return "", err
	}

	err = cmd.Wait()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, stderr)
	}

	container := strings.TrimSpace(string(stdout))
	return container, nil
}

func dockerExport(container string) ([]byte, error) {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return []byte{}, errors.New("Docker does not seem to be installed")
	}
	args := []string{"export", container}
	cmd := exec.Command(docker, args...)

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return []byte{}, err
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return []byte{}, err
	}

	err = cmd.Start()
	if err != nil {
		return []byte{}, err
	}

	stdout, err := ioutil.ReadAll(stdoutPipe)
	if err != nil {
		return []byte{}, err
	}

	stderr, err := ioutil.ReadAll(stderrPipe)
	if err != nil {
		return []byte{}, err
	}

	err = cmd.Wait()
	if err != nil {
		return []byte{}, fmt.Errorf("%s: %s", err, stderr)
	}

	return stdout, nil
}

func dockerRm(container string) error {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return errors.New("Docker does not seem to be installed")
	}
	args := []string{"rm", container}
	cmd := exec.Command(docker, args...)

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	_, err = ioutil.ReadAll(stdoutPipe)
	if err != nil {
		return err
	}

	stderr, err := ioutil.ReadAll(stderrPipe)
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("%s: %s", err, stderr)
	}

	return nil
}

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

func imageExtract(image string) ([]byte, error) {
	container, err := dockerCreate(image)
	if err != nil {
		return []byte{}, fmt.Errorf("Failed to docker create image %s: %v", image, err)
	}
	contents, err := dockerExport(container)
	if err != nil {
		return []byte{}, fmt.Errorf("Failed to docker export container from container %s: %v", container, err)
	}
	err = dockerRm(container)
	if err != nil {
		return []byte{}, fmt.Errorf("Failed to docker rm container %s: %v", container, err)
	}

	// now we need to filter out some files from the resulting tar archive
	out := new(bytes.Buffer)
	tw := tar.NewWriter(out)

	r := bytes.NewReader(contents)
	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return []byte{}, err
		}
		if exclude[hdr.Name] {
			io.Copy(ioutil.Discard, tr)
		} else if replace[hdr.Name] != "" {
			hdr.Size = int64(len(replace[hdr.Name]))
			tw.WriteHeader(hdr)
			buf := bytes.NewBufferString(replace[hdr.Name])
			io.Copy(tw, buf)
		} else {
			tw.WriteHeader(hdr)
			io.Copy(tw, tr)
		}
	}
	err = tw.Close()
	if err != nil {
		return []byte{}, err
	}
	return out.Bytes(), nil
}
