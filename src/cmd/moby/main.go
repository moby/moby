package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
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

func main() {
	flag.Usage = func() {
		fmt.Printf("USAGE: %s COMMAND\n\n", os.Args[0])
		fmt.Printf("Commands:\n")
		fmt.Printf("  build       Build a Moby image from a YAML file\n")
		fmt.Printf("  help        Print this message\n")
		fmt.Printf("\n")
		fmt.Printf("Run '%s COMMAND --help' for more information on the command\n", os.Args[0])
	}

	buildCmd := flag.NewFlagSet("build", flag.ExitOnError)
	buildCmd.Usage = func() {
		fmt.Printf("USAGE: %s build [options] [file.yaml]\n\n", os.Args[0])
		fmt.Printf("'file.yaml' defaults to 'moby.yaml' if not specified.\n\n")
		fmt.Printf("Options:\n")
		buildCmd.PrintDefaults()
	}
	buildName := buildCmd.String("name", "", "Name to use for output files")

	if len(os.Args) < 2 {
		fmt.Printf("Please specify a command.\n\n")
		flag.Usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "build":
		buildCmd.Parse(os.Args[2:])
		build(*buildName, buildCmd.Args())
	case "help":
		flag.Usage()
	default:
		fmt.Printf("%q is not valid command.\n\n", os.Args[1])
		flag.Usage()
		os.Exit(1)
	}
}
