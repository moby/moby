package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/docker/docker/pkg/homedir"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

func runSetupCmd(cmd *cobra.Command, args []string) {
	check()
	if err := setup(); err != nil {
		logrus.Fatal(err)
	}
}

func setup() error {
	// detect variables
	binDir, err := detectBinDir()
	if err != nil {
		return err
	}
	u, err := user.Current()
	if err != nil {
		return err
	}
	xrd := os.Getenv("XDG_RUNTIME_DIR")
	xrdCreated := false
	if err := unix.Access(xrd, unix.W_OK); err != nil {
		xrd := fmt.Sprintf("/tmp/docker-%s", u.Uid)
		if err := os.MkdirAll(xrd, 0700); err != nil {
			return err
		}
		xrdCreated = true
	}

	// setup systemd unit if running with systemd user instance,
	// otherwise just show command instruction
	if userSystemdAvailable() {
		if err := setupSystemd(binDir, xrd, u); err != nil {
			return err
		}
	} else {
		if err := setupNonSystemd(binDir); err != nil {
			return err
		}
	}

	// Print ~/.bashrc instruction
	inst := "Make sure the following environment variables are set (or add them to ~/.bashrc):\n"
	if xrdCreated {
		inst += fmt.Sprintf("export XDG_RUNTIME_DIR=%s\n", xrd)
	}
	inst += fmt.Sprintf("export PATH=%s:$PATH\n", binDir)
	inst += fmt.Sprintf("export DOCKER_HOST=unix://%s/docker.sock\n", xrd)
	logrus.Info(inst)
	return nil
}

func setupNonSystemd(binDir string) error {
	inst := "systemd (user instance) not detected, daemon needs to be started manually:\n"
	inst += fmt.Sprintf("export PATH=%s:/sbin:/usr/sbin:$PATH\n", binDir)
	inst += fmt.Sprintf("%s/%s", binDir, dockerdRootlessSh)
	if skipIptables {
		inst += "--iptables=false"
	}
	inst += "\n"
	logrus.Warn(inst)
	return nil
}

func setupSystemd(binDir, xdgRuntimeDir string, u *user.User) error {
	// cfgHome is typically ~/.config
	cfgHome, err := homedir.GetConfigHome()
	if err != nil {
		return err
	}
	systemdUserDir := filepath.Join(cfgHome, "systemd/user")
	if err := os.MkdirAll(systemdUserDir, 0755); err != nil {
		return err
	}
	// ~/.config/systemd/user/docker.service
	dockerServiceFile := filepath.Join(systemdUserDir, systemdUnit)
	if _, err := os.Stat(dockerServiceFile); err == nil {
		logrus.Infof("File already exists: %s", dockerServiceFile)
	} else {
		t, err := template.New(systemdUnit).Parse(systemdTemplate)
		if err != nil {
			return err
		}
		opts := SystemdTemplateOpts{
			BinDir:            binDir,
			DockerdRootlessSh: dockerdRootlessSh,
		}
		if skipIptables {
			opts.Flags = "--iptables=false"
		}
		logrus.Infof("Installing systemd unit file %s", dockerServiceFile)
		w, err := os.Create(dockerServiceFile)
		if err != nil {
			return err
		}
		err = t.Execute(w, &opts)
		w.Close()
		if err != nil {
			return err
		}
	}
	if err := cmd("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return err
	}
	if err := cmd("systemctl", "--user", "--no-pager", "--full", "status", systemdUnit).Run(); err != nil {
		logrus.Infof("Starting %s", systemdUnit)
		if err := cmd("systemctl", "--user", "start", systemdUnit).Run(); err != nil {
			return err
		}
		time.Sleep(3 * time.Second)
		if err := cmd("systemctl", "--user", "--no-pager", "--full", "status", systemdUnit).Run(); err != nil {
			return err
		}
	}
	dockerVersionCmd := cmd(filepath.Join(binDir, "docker"), "version")
	dockerVersionCmd.Env = append(os.Environ(), fmt.Sprintf("DOCKER_HOST=unix://%s/docker.sock", xdgRuntimeDir))
	if err := dockerVersionCmd.Run(); err != nil {
		return err
	}
	if err := cmd("systemctl", "--user", "enable", systemdUnit).Run(); err != nil {
		return err
	}
	logrus.Info("================================================================================")
	logrus.Infof("Installed %s successfully", systemdUnit)
	logrus.Infof("To control docker service, run: systemctl --user (start|stop|restart) %s", systemdUnit)
	logrus.Infof("To run docker service on system startup, run: sudo loginctl enable-linger %s", u.Username)
	return nil
}

func cmd(args ...string) *exec.Cmd {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logrus.Infof("+ %s", strings.Join(args, " "))
	return cmd
}
