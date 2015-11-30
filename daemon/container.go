package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/opencontainers/runc/libcontainer/label"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/exec"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/jsonfilelog"
	"github.com/docker/docker/daemon/network"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/nat"
	"github.com/docker/docker/pkg/promise"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/symlink"

	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/volume"
)

const configFileName = "config.v2.json"

var (
	// ErrRootFSReadOnly is returned when a container
	// rootfs is marked readonly.
	ErrRootFSReadOnly = errors.New("container rootfs is marked read-only")
)

// CommonContainer holds the fields for a container which are
// applicable across all platforms supported by the daemon.
type CommonContainer struct {
	*runconfig.StreamConfig
	// embed for Container to support states directly.
	*State          `json:"State"` // Needed for remote api version <= 1.11
	root            string         // Path to the "home" of the container, including metadata.
	basefs          string         // Path to the graphdriver mountpoint
	rwlayer         layer.RWLayer
	ID              string
	Created         time.Time
	Path            string
	Args            []string
	Config          *runconfig.Config
	ImageID         image.ID `json:"Image"`
	NetworkSettings *network.Settings
	LogPath         string
	Name            string
	Driver          string
	// MountLabel contains the options for the 'mount' command
	MountLabel             string
	ProcessLabel           string
	RestartCount           int
	HasBeenStartedBefore   bool
	HasBeenManuallyStopped bool // used for unless-stopped restart policy
	MountPoints            map[string]*volume.MountPoint
	hostConfig             *runconfig.HostConfig
	command                *execdriver.Command
	monitor                *containerMonitor
	execCommands           *exec.Store
	// logDriver for closing
	logDriver logger.Logger
	logCopier *logger.Copier
}

// newBaseContainer creates a new container with its
// basic configuration.
func newBaseContainer(id, root string) *Container {
	return &Container{
		CommonContainer: CommonContainer{
			ID:           id,
			State:        NewState(),
			execCommands: exec.NewStore(),
			root:         root,
			MountPoints:  make(map[string]*volume.MountPoint),
			StreamConfig: runconfig.NewStreamConfig(),
		},
	}
}

func (container *Container) fromDisk() error {
	pth, err := container.jsonPath()
	if err != nil {
		return err
	}

	jsonSource, err := os.Open(pth)
	if err != nil {
		return err
	}
	defer jsonSource.Close()

	dec := json.NewDecoder(jsonSource)

	// Load container settings
	if err := dec.Decode(container); err != nil {
		return err
	}

	if err := label.ReserveLabel(container.ProcessLabel); err != nil {
		return err
	}
	return container.readHostConfig()
}

func (container *Container) toDisk() error {
	pth, err := container.jsonPath()
	if err != nil {
		return err
	}

	jsonSource, err := os.Create(pth)
	if err != nil {
		return err
	}
	defer jsonSource.Close()

	enc := json.NewEncoder(jsonSource)

	// Save container settings
	if err := enc.Encode(container); err != nil {
		return err
	}

	return container.writeHostConfig()
}

func (container *Container) toDiskLocking() error {
	container.Lock()
	err := container.toDisk()
	container.Unlock()
	return err
}

func (container *Container) readHostConfig() error {
	container.hostConfig = &runconfig.HostConfig{}
	// If the hostconfig file does not exist, do not read it.
	// (We still have to initialize container.hostConfig,
	// but that's OK, since we just did that above.)
	pth, err := container.hostConfigPath()
	if err != nil {
		return err
	}

	f, err := os.Open(pth)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&container.hostConfig); err != nil {
		return err
	}

	initDNSHostConfig(container)

	return nil
}

func (container *Container) writeHostConfig() error {
	pth, err := container.hostConfigPath()
	if err != nil {
		return err
	}

	f, err := os.Create(pth)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(&container.hostConfig)
}

// GetResourcePath evaluates `path` in the scope of the container's basefs, with proper path
// sanitisation. Symlinks are all scoped to the basefs of the container, as
// though the container's basefs was `/`.
//
// The basefs of a container is the host-facing path which is bind-mounted as
// `/` inside the container. This method is essentially used to access a
// particular path inside the container as though you were a process in that
// container.
//
// NOTE: The returned path is *only* safely scoped inside the container's basefs
//       if no component of the returned path changes (such as a component
//       symlinking to a different path) between using this method and using the
//       path. See symlink.FollowSymlinkInScope for more details.
func (container *Container) GetResourcePath(path string) (string, error) {
	// IMPORTANT - These are paths on the OS where the daemon is running, hence
	// any filepath operations must be done in an OS agnostic way.
	cleanPath := filepath.Join(string(os.PathSeparator), path)
	r, e := symlink.FollowSymlinkInScope(filepath.Join(container.basefs, cleanPath), container.basefs)
	return r, e
}

// Evaluates `path` in the scope of the container's root, with proper path
// sanitisation. Symlinks are all scoped to the root of the container, as
// though the container's root was `/`.
//
// The root of a container is the host-facing configuration metadata directory.
// Only use this method to safely access the container's `container.json` or
// other metadata files. If in doubt, use container.GetResourcePath.
//
// NOTE: The returned path is *only* safely scoped inside the container's root
//       if no component of the returned path changes (such as a component
//       symlinking to a different path) between using this method and using the
//       path. See symlink.FollowSymlinkInScope for more details.
func (container *Container) getRootResourcePath(path string) (string, error) {
	// IMPORTANT - These are paths on the OS where the daemon is running, hence
	// any filepath operations must be done in an OS agnostic way.
	cleanPath := filepath.Join(string(os.PathSeparator), path)
	return symlink.FollowSymlinkInScope(filepath.Join(container.root, cleanPath), container.root)
}

// ExitOnNext signals to the monitor that it should not restart the container
// after we send the kill signal.
func (container *Container) ExitOnNext() {
	container.monitor.ExitOnNext()
}

// Resize changes the TTY of the process running inside the container
// to the given height and width. The container must be running.
func (container *Container) Resize(h, w int) error {
	if err := container.command.ProcessConfig.Terminal.Resize(h, w); err != nil {
		return err
	}
	return nil
}

func (container *Container) hostConfigPath() (string, error) {
	return container.getRootResourcePath("hostconfig.json")
}

func (container *Container) jsonPath() (string, error) {
	return container.getRootResourcePath(configFileName)
}

// This directory is only usable when the container is running
func (container *Container) rootfsPath() string {
	return container.basefs
}

func validateID(id string) error {
	if id == "" {
		return derr.ErrorCodeEmptyID
	}
	return nil
}

// Returns true if the container exposes a certain port
func (container *Container) exposes(p nat.Port) bool {
	_, exists := container.Config.ExposedPorts[p]
	return exists
}

func (container *Container) getLogConfig(defaultConfig runconfig.LogConfig) runconfig.LogConfig {
	cfg := container.hostConfig.LogConfig
	if cfg.Type != "" || len(cfg.Config) > 0 { // container has log driver configured
		if cfg.Type == "" {
			cfg.Type = jsonfilelog.Name
		}
		return cfg
	}
	// Use daemon's default log config for containers
	return defaultConfig
}

// StartLogger starts a new logger driver for the container.
func (container *Container) StartLogger(cfg runconfig.LogConfig) (logger.Logger, error) {
	c, err := logger.GetLogDriver(cfg.Type)
	if err != nil {
		return nil, derr.ErrorCodeLoggingFactory.WithArgs(err)
	}
	ctx := logger.Context{
		Config:              cfg.Config,
		ContainerID:         container.ID,
		ContainerName:       container.Name,
		ContainerEntrypoint: container.Path,
		ContainerArgs:       container.Args,
		ContainerImageID:    container.ImageID.String(),
		ContainerImageName:  container.Config.Image,
		ContainerCreated:    container.Created,
		ContainerEnv:        container.Config.Env,
		ContainerLabels:     container.Config.Labels,
	}

	// Set logging file for "json-logger"
	if cfg.Type == jsonfilelog.Name {
		ctx.LogPath, err = container.getRootResourcePath(fmt.Sprintf("%s-json.log", container.ID))
		if err != nil {
			return nil, err
		}
	}
	return c(ctx)
}

func (container *Container) getProcessLabel() string {
	// even if we have a process label return "" if we are running
	// in privileged mode
	if container.hostConfig.Privileged {
		return ""
	}
	return container.ProcessLabel
}

func (container *Container) getMountLabel() string {
	if container.hostConfig.Privileged {
		return ""
	}
	return container.MountLabel
}

func (container *Container) getExecIDs() []string {
	return container.execCommands.List()
}

// Attach connects to the container's TTY, delegating to standard
// streams or websockets depending on the configuration.
func (container *Container) Attach(stdin io.ReadCloser, stdout io.Writer, stderr io.Writer) chan error {
	return attach(container.StreamConfig, container.Config.OpenStdin, container.Config.StdinOnce, container.Config.Tty, stdin, stdout, stderr)
}

func attach(streamConfig *runconfig.StreamConfig, openStdin, stdinOnce, tty bool, stdin io.ReadCloser, stdout io.Writer, stderr io.Writer) chan error {
	var (
		cStdout, cStderr io.ReadCloser
		cStdin           io.WriteCloser
		wg               sync.WaitGroup
		errors           = make(chan error, 3)
	)

	if stdin != nil && openStdin {
		cStdin = streamConfig.StdinPipe()
		wg.Add(1)
	}

	if stdout != nil {
		cStdout = streamConfig.StdoutPipe()
		wg.Add(1)
	}

	if stderr != nil {
		cStderr = streamConfig.StderrPipe()
		wg.Add(1)
	}

	// Connect stdin of container to the http conn.
	go func() {
		if stdin == nil || !openStdin {
			return
		}
		logrus.Debugf("attach: stdin: begin")
		defer func() {
			if stdinOnce && !tty {
				cStdin.Close()
			} else {
				// No matter what, when stdin is closed (io.Copy unblock), close stdout and stderr
				if cStdout != nil {
					cStdout.Close()
				}
				if cStderr != nil {
					cStderr.Close()
				}
			}
			wg.Done()
			logrus.Debugf("attach: stdin: end")
		}()

		var err error
		if tty {
			_, err = copyEscapable(cStdin, stdin)
		} else {
			_, err = io.Copy(cStdin, stdin)

		}
		if err == io.ErrClosedPipe {
			err = nil
		}
		if err != nil {
			logrus.Errorf("attach: stdin: %s", err)
			errors <- err
			return
		}
	}()

	attachStream := func(name string, stream io.Writer, streamPipe io.ReadCloser) {
		if stream == nil {
			return
		}
		defer func() {
			// Make sure stdin gets closed
			if stdin != nil {
				stdin.Close()
			}
			streamPipe.Close()
			wg.Done()
			logrus.Debugf("attach: %s: end", name)
		}()

		logrus.Debugf("attach: %s: begin", name)
		_, err := io.Copy(stream, streamPipe)
		if err == io.ErrClosedPipe {
			err = nil
		}
		if err != nil {
			logrus.Errorf("attach: %s: %v", name, err)
			errors <- err
		}
	}

	go attachStream("stdout", stdout, cStdout)
	go attachStream("stderr", stderr, cStderr)

	return promise.Go(func() error {
		wg.Wait()
		close(errors)
		for err := range errors {
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Code c/c from io.Copy() modified to handle escape sequence
func copyEscapable(dst io.Writer, src io.ReadCloser) (written int64, err error) {
	buf := make([]byte, 32*1024)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			// ---- Docker addition
			// char 16 is C-p
			if nr == 1 && buf[0] == 16 {
				nr, er = src.Read(buf)
				// char 17 is C-q
				if nr == 1 && buf[0] == 17 {
					if err := src.Close(); err != nil {
						return 0, err
					}
					return 0, nil
				}
			}
			// ---- End of docker
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
	}
	return written, err
}

func (container *Container) shouldRestart() bool {
	return container.hostConfig.RestartPolicy.Name == "always" ||
		(container.hostConfig.RestartPolicy.Name == "unless-stopped" && !container.HasBeenManuallyStopped) ||
		(container.hostConfig.RestartPolicy.Name == "on-failure" && container.ExitCode != 0)
}

func (container *Container) addBindMountPoint(name, source, destination string, rw bool) {
	container.MountPoints[destination] = &volume.MountPoint{
		Name:        name,
		Source:      source,
		Destination: destination,
		RW:          rw,
	}
}

func (container *Container) addLocalMountPoint(name, destination string, rw bool) {
	container.MountPoints[destination] = &volume.MountPoint{
		Name:        name,
		Driver:      volume.DefaultDriverName,
		Destination: destination,
		RW:          rw,
	}
}

func (container *Container) addMountPointWithVolume(destination string, vol volume.Volume, rw bool) {
	container.MountPoints[destination] = &volume.MountPoint{
		Name:        vol.Name(),
		Driver:      vol.DriverName(),
		Destination: destination,
		RW:          rw,
		Volume:      vol,
	}
}

func (container *Container) isDestinationMounted(destination string) bool {
	return container.MountPoints[destination] != nil
}

func (container *Container) stopSignal() int {
	var stopSignal syscall.Signal
	if container.Config.StopSignal != "" {
		stopSignal, _ = signal.ParseSignal(container.Config.StopSignal)
	}

	if int(stopSignal) == 0 {
		stopSignal, _ = signal.ParseSignal(signal.DefaultStopSignal)
	}
	return int(stopSignal)
}

// initDNSHostConfig ensures that the dns fields are never nil.
// New containers don't ever have those fields nil,
// but pre created containers can still have those nil values.
// The non-recommended host configuration in the start api can
// make these fields nil again, this corrects that issue until
// we remove that behavior for good.
// See https://github.com/docker/docker/pull/17779
// for a more detailed explanation on why we don't want that.
func initDNSHostConfig(container *Container) {
	if container.hostConfig.DNS == nil {
		container.hostConfig.DNS = make([]string, 0)
	}

	if container.hostConfig.DNSSearch == nil {
		container.hostConfig.DNSSearch = make([]string, 0)
	}

	if container.hostConfig.DNSOptions == nil {
		container.hostConfig.DNSOptions = make([]string, 0)
	}
}
