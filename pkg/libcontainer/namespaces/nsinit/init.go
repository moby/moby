package nsinit

import (
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

// InitNamespace should be run inside an existing namespace to setup
// common mounts, drop capabilities, and setup network interfaces
func InitNamespace(container *libcontainer.Container) error {
	println("|||||||||||||")
	if err := setLogFile(container); err != nil {
		return err
	}
	println(container.LogFile)
	log.Printf("--------->")
	rootfs, err := resolveRootfs(container)
	if err != nil {
		return err
	}

	// any errors encoutered inside the namespace we should write
	// out to a log or a pipe to our parent and exit(1)
	// because writing to stderr will not work after we close
	if err := closeMasterAndStd(os.NewFile(container.Master, "/dev/ptmx")); err != nil {
		log.Fatalf("close master and std %s", err)
		return err
	}

	slave, err := openTerminal(container.Console, syscall.O_RDWR)
	if err != nil {
		log.Fatalf("open terminal %s", err)
		return err
	}
	if err := dupSlave(slave); err != nil {
		log.Fatalf("dup2 slave %s", err)
		return err
	}

	/*
		if container.NetNsFd > 0 {
			if err := joinExistingNamespace(container.NetNsFd, libcontainer.CLONE_NEWNET); err != nil {
				log.Fatalf("join existing net namespace %s", err)
			}
		}
	*/

	if _, err := system.Setsid(); err != nil {
		log.Fatalf("setsid %s", err)
		return err
	}
	if err := system.Setctty(); err != nil {
		log.Fatalf("setctty %s", err)
		return err
	}
	if err := system.ParentDeathSignal(); err != nil {
		log.Fatalf("parent deth signal %s", err)
		return err
	}
	if err := namespaces.SetupNewMountNamespace(rootfs, container.Console, container.ReadonlyFs); err != nil {
		log.Fatalf("setup mount namespace %s", err)
		return err
	}
	if err := system.Sethostname(container.ID); err != nil {
		log.Fatalf("sethostname %s", err)
		return err
	}
	if err := capabilities.DropCapabilities(container); err != nil {
		log.Fatalf("drop capabilities %s", err)
		return err
	}
	if err := setupUser(container); err != nil {
		log.Fatalf("setup user %s", err)
		return err
	}
	if container.WorkingDir != "" {
		if err := system.Chdir(container.WorkingDir); err != nil {
			log.Fatalf("chdir to %s %s", container.WorkingDir, err)
			return err
		}
	}
	if err := system.Exec(container.Command.Args[0], container.Command.Args[0:], container.Command.Env); err != nil {
		log.Fatalf("exec %s", err)
		return err
	}
	panic("unreachable")
}

func resolveRootfs(container *libcontainer.Container) (string, error) {
	rootfs, err := filepath.Abs(container.RootFs)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(rootfs)
}

func closeMasterAndStd(master *os.File) error {
	master.Close()
	os.Stdin.Close()
	os.Stdout.Close()
	os.Stderr.Close()
	return nil
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
