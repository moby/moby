package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os/exec"
	"path/filepath"
)

func dockerRun(args ...string) ([]byte, error) {
	// TODO switch to using Docker client API not exec - just a quick prototype
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
		return []byte{}, fmt.Errorf("%s: %s", err, stderr)
	}

	return stdout, nil
}

func dockerRunInput(input io.Reader, args ...string) ([]byte, error) {
	// TODO switch to using Docker client API not exec - just a quick prototype
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
		return []byte{}, fmt.Errorf("%s: %s", err, stderr)
	}

	return stdout, nil
}

var (
	conf string
	name string
)

func main() {
	flag.StringVar(&name, "name", "", "Name to use for output files")
	flag.Parse()

	conf = "moby.yaml"
	if len(flag.Args()) > 0 {
		conf = flag.Args()[0]
	}

	if name == "" {
		name = filepath.Base(conf)
		ext := filepath.Ext(conf)
		if ext != "" {
			name = name[:len(name)-len(ext)]
		}
	}

	config, err := ioutil.ReadFile(conf)
	if err != nil {
		log.Fatalf("Cannot open config file: %v", err)
	}

	m, err := NewConfig(config)
	if err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	build(m, name)
}
