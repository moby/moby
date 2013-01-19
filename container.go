package docker

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"syscall"
)

type Container struct {
	Name string
	Root string
	Path string
	Args []string

	*Config
	*Filesystem
	*State

	lxcConfigPath string
	cmd           *exec.Cmd
	stdout        *writeBroadcaster
	stderr        *writeBroadcaster
}

type Config struct {
	Hostname string
	Ram      int64
}

func createContainer(name string, root string, command string, args []string, layers []string, config *Config) (*Container, error) {
	container := &Container{
		Name:       name,
		Root:       root,
		Path:       command,
		Args:       args,
		Config:     config,
		Filesystem: newFilesystem(path.Join(root, "rootfs"), path.Join(root, "rw"), layers),
		State:      newState(),

		lxcConfigPath: path.Join(root, "config.lxc"),
		stdout:        newWriteBroadcaster(),
		stderr:        newWriteBroadcaster(),
	}

	if err := os.Mkdir(root, 0700); err != nil {
		return nil, err
	}

	if err := container.save(); err != nil {
		return nil, err
	}
	if err := container.generateLXCConfig(); err != nil {
		return nil, err
	}
	return container, nil
}

func loadContainer(containerPath string) (*Container, error) {
	configPath := path.Join(containerPath, "config.json")
	fi, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer fi.Close()
	enc := json.NewDecoder(fi)
	container := &Container{}
	if err := enc.Decode(container); err != nil {
		return nil, err
	}
	return container, nil
}

func (container *Container) save() error {
	configPath := path.Join(container.Root, "config.json")
	fo, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer fo.Close()
	enc := json.NewEncoder(fo)
	if err := enc.Encode(container); err != nil {
		return err
	}
	return nil
}

func (container *Container) generateLXCConfig() error {
	fo, err := os.Create(container.lxcConfigPath)
	if err != nil {
		return err
	}
	defer fo.Close()

	if err := LxcTemplateCompiled.Execute(fo, container); err != nil {
		return err
	}
	return nil
}

func (container *Container) Start() error {
	if err := container.Filesystem.Mount(); err != nil {
		return err
	}

	params := []string{
		"-n", container.Name,
		"-f", container.lxcConfigPath,
		"--",
		container.Path,
	}
	params = append(params, container.Args...)

	container.cmd = exec.Command("/usr/bin/lxc-start", params...)
	container.cmd.Stdout = container.stdout
	container.cmd.Stderr = container.stderr

	if err := container.cmd.Start(); err != nil {
		return err
	}
	container.State.setRunning(container.cmd.Process.Pid)
	go container.monitor()

	// Wait until we are out of the STARTING state before returning
	//
	// Even though lxc-wait blocks until the container reaches a given state,
	// sometimes it returns an error code, which is why we have to retry.
	//
	// This is a rare race condition that happens for short lived programs
	for retries := 0; retries < 3; retries++ {
		err := exec.Command("/usr/bin/lxc-wait", "-n", container.Name, "-s", "RUNNING|STOPPED").Run()
		if err == nil {
			return nil
		}
	}
	return errors.New("Container failed to start")
}

func (container *Container) Run() error {
	if err := container.Start(); err != nil {
		return err
	}
	container.Wait()
	return nil
}

func (container *Container) Output() (output []byte, err error) {
	pipe, err := container.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer pipe.Close()
	if err := container.Start(); err != nil {
		return nil, err
	}
	output, err = ioutil.ReadAll(pipe)
	container.Wait()
	return output, err
}

func (container *Container) StdoutPipe() (io.ReadCloser, error) {
	reader, writer := io.Pipe()
	container.stdout.AddWriter(writer)
	return newBufReader(reader), nil
}

func (container *Container) StderrPipe() (io.ReadCloser, error) {
	reader, writer := io.Pipe()
	container.stderr.AddWriter(writer)
	return newBufReader(reader), nil
}

func (container *Container) monitor() {
	container.cmd.Wait()
	container.stdout.Close()
	container.stderr.Close()
	container.State.setStopped(container.cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus())
}

func (container *Container) Stop() error {
	if container.State.Running {
		if err := exec.Command("/usr/bin/lxc-stop", "-n", container.Name).Run(); err != nil {
			return err
		}
		//FIXME: We should lxc-wait for the container to stop
	}

	if err := container.Filesystem.Umount(); err != nil {
		// FIXME: Do not abort, probably already umounted?
		return nil
	}
	return nil
}

func (container *Container) Wait() {
	for container.State.Running {
		container.State.wait()
	}
}
