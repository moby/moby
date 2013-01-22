package docker

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"syscall"
	"time"
)

type Container struct {
	Id   string
	Root string

	Created time.Time

	Path string
	Args []string

	Config     *Config
	Filesystem *Filesystem
	State      *State

	lxcConfigPath string
	cmd           *exec.Cmd
	stdout        *writeBroadcaster
	stderr        *writeBroadcaster
}

type Config struct {
	Hostname string
	Ram      int64
}

func createContainer(id string, root string, command string, args []string, layers []string, config *Config) (*Container, error) {
	container := &Container{
		Id:         id,
		Root:       root,
		Created:    time.Now(),
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
	data, err := ioutil.ReadFile(path.Join(containerPath, "config.json"))
	if err != nil {
		return nil, err
	}
	var container *Container
	if err := json.Unmarshal(data, container); err != nil {
		return nil, err
	}
	return container, nil
}

func (container *Container) save() (err error) {
	data, err := json.Marshal(container)
	if err != nil {
		return
	}
	return ioutil.WriteFile(path.Join(container.Root, "config.json"), data, 0700)
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
		"-n", container.Id,
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
	container.save()
	go container.monitor()

	// Wait until we are out of the STARTING state before returning
	//
	// Even though lxc-wait blocks until the container reaches a given state,
	// sometimes it returns an error code, which is why we have to retry.
	//
	// This is a rare race condition that happens for short lived programs
	for retries := 0; retries < 3; retries++ {
		err := exec.Command("/usr/bin/lxc-wait", "-n", container.Id, "-s", "RUNNING|STOPPED").Run()
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
	// Wait for the program to exit
	container.cmd.Wait()

	// Cleanup
	container.stdout.Close()
	container.stderr.Close()
	if err := container.Filesystem.Umount(); err != nil {
		log.Printf("%v: Failed to umount filesystem: %v", container.Id, err)
	}

	// Report status back
	exitCode := container.cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
	container.State.setStopped(exitCode)
	container.save()
}

func (container *Container) kill() error {
	// This will cause the main container process to receive a SIGKILL
	if err := exec.Command("/usr/bin/lxc-stop", "-n", container.Id).Run(); err != nil {
		return err
	}

	// Wait for the container to be actually stopped
	if err := exec.Command("/usr/bin/lxc-wait", "-n", container.Id, "-s", "STOPPED").Run(); err != nil {
		return err
	}
	return nil
}

func (container *Container) Kill() error {
	if !container.State.Running {
		return nil
	}
	return container.kill()
}

func (container *Container) Stop() error {
	if !container.State.Running {
		return nil
	}

	// 1. Send a SIGTERM
	if err := exec.Command("/usr/bin/lxc-kill", "-n", container.Id, "15").Run(); err != nil {
		return err
	}

	// 2. Wait for the process to exit on its own
	if err := container.WaitTimeout(10 * time.Second); err != nil {
		log.Printf("Container %v failed to exit within 10 seconds of SIGTERM", container.Id)
	}

	// 3. Force kill
	if err := container.kill(); err != nil {
		return err
	}
	return nil
}

func (container *Container) Restart() error {
	if err := container.Stop(); err != nil {
		return err
	}
	if err := container.Start(); err != nil {
		return err
	}
	return nil
}

func (container *Container) Wait() {
	for container.State.Running {
		container.State.wait()
	}
}

func (container *Container) WaitTimeout(timeout time.Duration) error {
	done := make(chan bool)
	go func() {
		container.Wait()
		done <- true
	}()

	select {
	case <-time.After(timeout):
		return errors.New("Timed Out")
	case <-done:
		return nil
	}
	return nil
}
