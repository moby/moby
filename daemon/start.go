package daemon

import (
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/docker/docker/runconfig"
)

func (daemon *Daemon) ContainerStart(name string, hostConfig *runconfig.HostConfig, out io.Writer, sdNotifyFlag bool) error {

	notifyChan := make(chan []byte)
	errChan := make(chan error)

	container, err := daemon.Get(name)
	if err != nil {
		return err
	}

	if container.IsPaused() {
		return fmt.Errorf("Cannot start a paused container, try unpause instead.")
	}

	if container.IsRunning() {
		return fmt.Errorf("Container already started")
	}

	if _, err = daemon.verifyContainerSettings(hostConfig, nil); err != nil {
		return err
	}

	// This is kept for backward compatibility - hostconfig should be passed when
	// creating a container, not during start.
	if hostConfig != nil {
		if err := daemon.setHostConfig(container, hostConfig); err != nil {
			return err
		}
	}

	if sdNotifyFlag {
		tmpNotifyDir := fmt.Sprintf("/run/docker/%s", container.ID[:10])
		defer os.RemoveAll(tmpNotifyDir)
		if err := setupSdNotify(container, tmpNotifyDir, notifyChan, errChan); err != nil {
			return err
		}
	}

	if err := container.Start(); err != nil {
		return fmt.Errorf("Cannot start container %s: %s", name, err)
	}

	if sdNotifyFlag {
		if err := waitOnSdNotify(notifyChan, errChan, out); err != nil {
			return err
		}
	}

	return nil
}

func setupSdNotify(container *Container, tmpNotifyDir string, notifyChan chan []byte, errChan chan error) error {

	if err := os.Mkdir(tmpNotifyDir, 0700); err != nil {
		return err
	}

	container.addBindMountPoint("", tmpNotifyDir, tmpNotifyDir, true)

	container.Config.Env = []string{fmt.Sprintf("NOTIFY_SOCKET=%s/sdNotifySock", tmpNotifyDir)}

	sock := &net.UnixAddr{fmt.Sprintf("%s/sdNotifySock", tmpNotifyDir), "unixgram"}

	go func(sock *net.UnixAddr) {

		conn, err := net.ListenUnixgram("unixgram", sock)
		if err != nil {
			errChan <- err
		}
		var buf [1024]byte
		for {
			n, err := conn.Read(buf[:])
			if err != nil {
				errChan <- err
			}
			if n > 0 {
				notifyChan <- buf[:n]
			}
		}
	}(sock)

	return nil
}

func waitOnSdNotify(notifyChan chan []byte, errChan chan error, out io.Writer) error {
	select {
	case <-time.After(60 * time.Second):
		return fmt.Errorf("sd_notify call timed out")
	case err := <-errChan:
		return err
	case notifyMessage := <-notifyChan:
		fmt.Fprintf(out, "%s", string(notifyMessage))
	}
	return nil
}
