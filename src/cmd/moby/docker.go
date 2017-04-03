package main

// We want to replace much of this with use of containerd tools
// and also using the Docker API not shelling out

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"golang.org/x/net/context"
)

func dockerRun(args ...string) ([]byte, error) {
	log.Debugf("docker run: %s", strings.Join(args, " "))
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

	log.Debugf("docker run: %s...Done", strings.Join(args, " "))
	return stdout, nil
}

func dockerRunInput(input io.Reader, args ...string) ([]byte, error) {
	log.Debugf("docker run (input): %s", strings.Join(args, " "))
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

	log.Debugf("docker run (input): %s...Done", strings.Join(args, " "))
	return stdout, nil
}

func dockerCreate(image string) (string, error) {
	log.Debugf("docker create: %s", image)
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
	log.Debugf("docker create: %s...Done", image)
	return container, nil
}

func dockerExport(container string) ([]byte, error) {
	log.Debugf("docker export: %s", container)
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

	log.Debugf("docker export: %s...Done", container)
	return stdout, nil
}

func dockerRm(container string) error {
	log.Debugf("docker rm: %s", container)
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

	log.Debugf("docker rm: %s...Done", container)
	return nil
}

func dockerPull(image string) error {
	log.Debugf("docker pull: %s", image)
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

	log.Debugf("docker pull: %s...Done", image)
	return nil
}

func dockerClient() (*client.Client, error) {
	// for maximum compatibility as we use nothing new
	err := os.Setenv("DOCKER_API_VERSION", "1.23")
	if err != nil {
		return nil, err
	}
	return client.NewEnvClient()
}

func dockerInspectImage(cli *client.Client, image string) (types.ImageInspect, error) {
	log.Debugf("docker inspect image: %s", image)

	inspect, _, err := cli.ImageInspectWithRaw(context.Background(), image, false)
	if err != nil {
		if client.IsErrImageNotFound(err) {
			pullErr := dockerPull(image)
			if pullErr != nil {
				return types.ImageInspect{}, pullErr
			}
			inspect, _, err = cli.ImageInspectWithRaw(context.Background(), image, false)
			if err != nil {
				return types.ImageInspect{}, err
			}
		} else {
			return types.ImageInspect{}, err
		}
	}

	log.Debugf("docker inspect image: %s...Done", image)

	return inspect, nil
}
