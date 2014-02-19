package main

import (
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/capabilities"
	"github.com/dotcloud/docker/pkg/libcontainer/namespaces"
	"github.com/dotcloud/docker/pkg/system"
	"log"
	"os"
	"path/filepath"
	"syscall"
)

func loadContainer() (*libcontainer.Container, error) {
	f, err := os.Open("container.json")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var container *libcontainer.Container
	if err := json.NewDecoder(f).Decode(&container); err != nil {
		return nil, err
	}
	return container, nil
}

func main() {
	container, err := loadContainer()
	if err != nil {
		log.Fatal(err)
	}

	if os.Args[1] == "exec" {
		_, err := namespaces.ExecContainer(container)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}
	console := os.Args[1]

	if err := setLogFile(container); err != nil {
		log.Fatal(err)
	}

	rootfs, err := resolveRootfs()
	if err != nil {
		log.Fatal(err)
	}

	// close pipes so that we can replace it with the pty
	os.Stdin.Close()
	os.Stdout.Close()
	os.Stderr.Close()

	slave, err := openTerminal(console, syscall.O_RDWR)
	if err != nil {
		log.Fatalf("open terminal %s", err)
	}
	if slave.Fd() != 0 {
		log.Fatalf("slave fd should be 0")
	}
	if err := dupSlave(slave); err != nil {
		log.Fatalf("dup2 slave %s", err)
	}

	if _, err := system.Setsid(); err != nil {
		log.Fatalf("setsid %s", err)
	}
	if err := system.Setctty(); err != nil {
		log.Fatalf("setctty %s", err)
	}
	if err := system.ParentDeathSignal(); err != nil {
		log.Fatalf("parent deth signal %s", err)
	}

	if err := setupNewMountNamespace(rootfs, console, container.ReadonlyFs); err != nil {
		log.Fatalf("setup mount namespace %s", err)
	}

	if container.Network != nil {
		if err := setupNetworking(container); err != nil {
			log.Fatalf("setup networking %s", err)
		}
	}

	if err := system.Sethostname(container.ID); err != nil {
		log.Fatalf("sethostname %s", err)
	}
	if err := capabilities.DropCapabilities(container); err != nil {
		log.Fatalf("drop capabilities %s", err)
	}
	if err := setupUser(container); err != nil {
		log.Fatalf("setup user %s", err)
	}
	if container.WorkingDir != "" {
		if err := system.Chdir(container.WorkingDir); err != nil {
			log.Fatalf("chdir to %s %s", container.WorkingDir, err)
		}
	}
	if err := system.Exec(container.Command.Args[0], container.Command.Args[0:], container.Command.Env); err != nil {
		log.Fatalf("exec %s", err)
	}
	panic("unreachable")
}

func resolveRootfs() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	rootfs, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(rootfs)
}

func setupUser(container *libcontainer.Container) error {
	// TODO: honor user passed on container
	if err := system.Setgroups(nil); err != nil {
		return err
	}
	if err := system.Setresgid(0, 0, 0); err != nil {
		return err
	}
	if err := system.Setresuid(0, 0, 0); err != nil {
		return err
	}
	return nil
}

func dupSlave(slave *os.File) error {
	// we close Stdin,etc so our pty slave should have fd 0
	if slave.Fd() != 0 {
		return fmt.Errorf("slave fd not 0 %d", slave.Fd())
	}
	if err := system.Dup2(slave.Fd(), 1); err != nil {
		return err
	}
	if err := system.Dup2(slave.Fd(), 2); err != nil {
		return err
	}
	return nil
}

// openTerminal is a clone of os.OpenFile without the O_CLOEXEC addition.
func openTerminal(name string, flag int) (*os.File, error) {
	r, e := syscall.Open(name, flag, 0)
	if e != nil {
		return nil, &os.PathError{"open", name, e}
	}
	return os.NewFile(uintptr(r), name), nil
}

func setLogFile(container *libcontainer.Container) error {
	f, err := os.OpenFile(container.LogFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0655)
	if err != nil {
		return err
	}
	log.SetOutput(f)
	return nil
}

func setupNetworking(conatiner *libcontainer.Container) error {
	return nil
}
