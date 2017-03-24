package main

// We want to replace much of this with use of containerd tools
// and also using the Docker API not shelling out

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"
)

func dockerRun(args ...string) ([]byte, error) {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return []byte{}, errors.New("Docker does not seem to be installed")
	}
	args = append([]string{"run", "--rm"}, args...)
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
		return []byte{}, fmt.Errorf("%v: %s", err, stderr)
	}

	return stdout, nil
}

func dockerRunInput(input io.Reader, args ...string) ([]byte, error) {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return []byte{}, errors.New("Docker does not seem to be installed")
	}
	args = append([]string{"run", "--rm", "-i"}, args...)
	cmd := exec.Command(docker, args...)
	cmd.Stdin = input

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
		return []byte{}, fmt.Errorf("%v: %s", err, stderr)
	}

	return stdout, nil
}

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
		return "", fmt.Errorf("%v: %s", err, stderr)
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
		return []byte{}, fmt.Errorf("%v: %s", err, stderr)
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
		return fmt.Errorf("%v: %s", err, stderr)
	}

	return nil
}

func dockerPull(image string) error {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return errors.New("Docker does not seem to be installed")
	}
	args := []string{"pull", image}
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
		return fmt.Errorf("%v: %s", err, stderr)
	}

	return nil
}
