// DEPRECATION NOTICE. PLEASE DO NOT ADD ANYTHING TO THIS FILE.
//
// For additional commments see server/server.go
//
package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/daemon"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/graph"
	"github.com/docker/docker/pkg/graphdb"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/tailfile"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

func (srv *Server) ContainerPause(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER", job.Name)
	}
	name := job.Args[0]
	container := srv.daemon.Get(name)
	if container == nil {
		return job.Errorf("No such container: %s", name)
	}
	if err := container.Pause(); err != nil {
		return job.Errorf("Cannot pause container %s: %s", name, err)
	}
	srv.LogEvent("pause", container.ID, srv.daemon.Repositories().ImageName(container.Image))
	return engine.StatusOK
}

func (srv *Server) ContainerUnpause(job *engine.Job) engine.Status {
	if n := len(job.Args); n < 1 || n > 2 {
		return job.Errorf("Usage: %s CONTAINER", job.Name)
	}
	name := job.Args[0]
	container := srv.daemon.Get(name)
	if container == nil {
		return job.Errorf("No such container: %s", name)
	}
	if err := container.Unpause(); err != nil {
		return job.Errorf("Cannot unpause container %s: %s", name, err)
	}
	srv.LogEvent("unpause", container.ID, srv.daemon.Repositories().ImageName(container.Image))
	return engine.StatusOK
}

// ContainerKill send signal to the container
// If no signal is given (sig 0), then Kill with SIGKILL and wait
// for the container to exit.
// If a signal is given, then just send it to the container and return.
func (srv *Server) ContainerKill(job *engine.Job) engine.Status {
	if n := len(job.Args); n < 1 || n > 2 {
		return job.Errorf("Usage: %s CONTAINER [SIGNAL]", job.Name)
	}
	var (
		name = job.Args[0]
		sig  uint64
		err  error
	)

	// If we have a signal, look at it. Otherwise, do nothing
	if len(job.Args) == 2 && job.Args[1] != "" {
		// Check if we passed the signal as a number:
		// The largest legal signal is 31, so let's parse on 5 bits
		sig, err = strconv.ParseUint(job.Args[1], 10, 5)
		if err != nil {
			// The signal is not a number, treat it as a string (either like "KILL" or like "SIGKILL")
			sig = uint64(signal.SignalMap[strings.TrimPrefix(job.Args[1], "SIG")])
		}

		if sig == 0 {
			return job.Errorf("Invalid signal: %s", job.Args[1])
		}
	}

	if container := srv.daemon.Get(name); container != nil {
		// If no signal is passed, or SIGKILL, perform regular Kill (SIGKILL + wait())
		if sig == 0 || syscall.Signal(sig) == syscall.SIGKILL {
			if err := container.Kill(); err != nil {
				return job.Errorf("Cannot kill container %s: %s", name, err)
			}
			srv.LogEvent("kill", container.ID, srv.daemon.Repositories().ImageName(container.Image))
		} else {
			// Otherwise, just send the requested signal
			if err := container.KillSig(int(sig)); err != nil {
				return job.Errorf("Cannot kill container %s: %s", name, err)
			}
			// FIXME: Add event for signals
		}
	} else {
		return job.Errorf("No such container: %s", name)
	}
	return engine.StatusOK
}

func (srv *Server) ContainerExport(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s container_id", job.Name)
	}
	name := job.Args[0]
	if container := srv.daemon.Get(name); container != nil {
		data, err := container.Export()
		if err != nil {
			return job.Errorf("%s: %s", name, err)
		}
		defer data.Close()

		// Stream the entire contents of the container (basically a volatile snapshot)
		if _, err := io.Copy(job.Stdout, data); err != nil {
			return job.Errorf("%s: %s", name, err)
		}
		// FIXME: factor job-specific LogEvent to engine.Job.Run()
		srv.LogEvent("export", container.ID, srv.daemon.Repositories().ImageName(container.Image))
		return engine.StatusOK
	}
	return job.Errorf("No such container: %s", name)
}

func (srv *Server) ContainerTop(job *engine.Job) engine.Status {
	if len(job.Args) != 1 && len(job.Args) != 2 {
		return job.Errorf("Not enough arguments. Usage: %s CONTAINER [PS_ARGS]\n", job.Name)
	}
	var (
		name   = job.Args[0]
		psArgs = "-ef"
	)

	if len(job.Args) == 2 && job.Args[1] != "" {
		psArgs = job.Args[1]
	}

	if container := srv.daemon.Get(name); container != nil {
		if !container.State.IsRunning() {
			return job.Errorf("Container %s is not running", name)
		}
		pids, err := srv.daemon.ExecutionDriver().GetPidsForContainer(container.ID)
		if err != nil {
			return job.Error(err)
		}
		output, err := exec.Command("ps", psArgs).Output()
		if err != nil {
			return job.Errorf("Error running ps: %s", err)
		}

		lines := strings.Split(string(output), "\n")
		header := strings.Fields(lines[0])
		out := &engine.Env{}
		out.SetList("Titles", header)

		pidIndex := -1
		for i, name := range header {
			if name == "PID" {
				pidIndex = i
			}
		}
		if pidIndex == -1 {
			return job.Errorf("Couldn't find PID field in ps output")
		}

		processes := [][]string{}
		for _, line := range lines[1:] {
			if len(line) == 0 {
				continue
			}
			fields := strings.Fields(line)
			p, err := strconv.Atoi(fields[pidIndex])
			if err != nil {
				return job.Errorf("Unexpected pid '%s': %s", fields[pidIndex], err)
			}

			for _, pid := range pids {
				if pid == p {
					// Make sure number of fields equals number of header titles
					// merging "overhanging" fields
					process := fields[:len(header)-1]
					process = append(process, strings.Join(fields[len(header)-1:], " "))
					processes = append(processes, process)
				}
			}
		}
		out.SetJson("Processes", processes)
		out.WriteTo(job.Stdout)
		return engine.StatusOK

	}
	return job.Errorf("No such container: %s", name)
}

func (srv *Server) ContainerChanges(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 1 {
		return job.Errorf("Usage: %s CONTAINER", job.Name)
	}
	name := job.Args[0]
	if container := srv.daemon.Get(name); container != nil {
		outs := engine.NewTable("", 0)
		changes, err := container.Changes()
		if err != nil {
			return job.Error(err)
		}
		for _, change := range changes {
			out := &engine.Env{}
			if err := out.Import(change); err != nil {
				return job.Error(err)
			}
			outs.Add(out)
		}
		if _, err := outs.WriteListTo(job.Stdout); err != nil {
			return job.Error(err)
		}
	} else {
		return job.Errorf("No such container: %s", name)
	}
	return engine.StatusOK
}

func (srv *Server) Containers(job *engine.Job) engine.Status {
	var (
		foundBefore bool
		displayed   int
		all         = job.GetenvBool("all")
		since       = job.Getenv("since")
		before      = job.Getenv("before")
		n           = job.GetenvInt("limit")
		size        = job.GetenvBool("size")
	)
	outs := engine.NewTable("Created", 0)

	names := map[string][]string{}
	srv.daemon.ContainerGraph().Walk("/", func(p string, e *graphdb.Entity) error {
		names[e.ID()] = append(names[e.ID()], p)
		return nil
	}, -1)

	var beforeCont, sinceCont *daemon.Container
	if before != "" {
		beforeCont = srv.daemon.Get(before)
		if beforeCont == nil {
			return job.Error(fmt.Errorf("Could not find container with name or id %s", before))
		}
	}

	if since != "" {
		sinceCont = srv.daemon.Get(since)
		if sinceCont == nil {
			return job.Error(fmt.Errorf("Could not find container with name or id %s", since))
		}
	}

	errLast := errors.New("last container")
	writeCont := func(container *daemon.Container) error {
		container.Lock()
		defer container.Unlock()
		if !container.State.IsRunning() && !all && n <= 0 && since == "" && before == "" {
			return nil
		}
		if before != "" && !foundBefore {
			if container.ID == beforeCont.ID {
				foundBefore = true
			}
			return nil
		}
		if n > 0 && displayed == n {
			return errLast
		}
		if since != "" {
			if container.ID == sinceCont.ID {
				return errLast
			}
		}
		displayed++
		out := &engine.Env{}
		out.Set("Id", container.ID)
		out.SetList("Names", names[container.ID])
		out.Set("Image", srv.daemon.Repositories().ImageName(container.Image))
		if len(container.Args) > 0 {
			args := []string{}
			for _, arg := range container.Args {
				if strings.Contains(arg, " ") {
					args = append(args, fmt.Sprintf("'%s'", arg))
				} else {
					args = append(args, arg)
				}
			}
			argsAsString := strings.Join(args, " ")

			out.Set("Command", fmt.Sprintf("\"%s %s\"", container.Path, argsAsString))
		} else {
			out.Set("Command", fmt.Sprintf("\"%s\"", container.Path))
		}
		out.SetInt64("Created", container.Created.Unix())
		out.Set("Status", container.State.String())
		str, err := container.NetworkSettings.PortMappingAPI().ToListString()
		if err != nil {
			return err
		}
		out.Set("Ports", str)
		if size {
			sizeRw, sizeRootFs := container.GetSize()
			out.SetInt64("SizeRw", sizeRw)
			out.SetInt64("SizeRootFs", sizeRootFs)
		}
		outs.Add(out)
		return nil
	}

	for _, container := range srv.daemon.List() {
		if err := writeCont(container); err != nil {
			if err != errLast {
				return job.Error(err)
			}
			break
		}
	}
	outs.ReverseSort()
	if _, err := outs.WriteListTo(job.Stdout); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

func (srv *Server) ContainerCommit(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Not enough arguments. Usage: %s CONTAINER\n", job.Name)
	}
	name := job.Args[0]

	container := srv.daemon.Get(name)
	if container == nil {
		return job.Errorf("No such container: %s", name)
	}

	var (
		config    = container.Config
		newConfig runconfig.Config
	)

	if err := job.GetenvJson("config", &newConfig); err != nil {
		return job.Error(err)
	}

	if err := runconfig.Merge(&newConfig, config); err != nil {
		return job.Error(err)
	}

	img, err := srv.daemon.Commit(container, job.Getenv("repo"), job.Getenv("tag"), job.Getenv("comment"), job.Getenv("author"), job.GetenvBool("pause"), &newConfig)
	if err != nil {
		return job.Error(err)
	}
	job.Printf("%s\n", img.ID)
	return engine.StatusOK
}

func (srv *Server) ContainerCreate(job *engine.Job) engine.Status {
	var name string
	if len(job.Args) == 1 {
		name = job.Args[0]
	} else if len(job.Args) > 1 {
		return job.Errorf("Usage: %s", job.Name)
	}
	config := runconfig.ContainerConfigFromJob(job)
	if config.Memory != 0 && config.Memory < 524288 {
		return job.Errorf("Minimum memory limit allowed is 512k")
	}
	if config.Memory > 0 && !srv.daemon.SystemConfig().MemoryLimit {
		job.Errorf("Your kernel does not support memory limit capabilities. Limitation discarded.\n")
		config.Memory = 0
	}
	if config.Memory > 0 && !srv.daemon.SystemConfig().SwapLimit {
		job.Errorf("Your kernel does not support swap limit capabilities. Limitation discarded.\n")
		config.MemorySwap = -1
	}
	container, buildWarnings, err := srv.daemon.Create(config, name)
	if err != nil {
		if srv.daemon.Graph().IsNotExist(err) {
			_, tag := parsers.ParseRepositoryTag(config.Image)
			if tag == "" {
				tag = graph.DEFAULTTAG
			}
			return job.Errorf("No such image: %s (tag: %s)", config.Image, tag)
		}
		return job.Error(err)
	}
	if !container.Config.NetworkDisabled && srv.daemon.SystemConfig().IPv4ForwardingDisabled {
		job.Errorf("IPv4 forwarding is disabled.\n")
	}
	srv.LogEvent("create", container.ID, srv.daemon.Repositories().ImageName(container.Image))
	// FIXME: this is necessary because daemon.Create might return a nil container
	// with a non-nil error. This should not happen! Once it's fixed we
	// can remove this workaround.
	if container != nil {
		job.Printf("%s\n", container.ID)
	}
	for _, warning := range buildWarnings {
		job.Errorf("%s\n", warning)
	}
	return engine.StatusOK
}

func (srv *Server) ContainerRestart(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}
	var (
		name = job.Args[0]
		t    = 10
	)
	if job.EnvExists("t") {
		t = job.GetenvInt("t")
	}
	if container := srv.daemon.Get(name); container != nil {
		if err := container.Restart(int(t)); err != nil {
			return job.Errorf("Cannot restart container %s: %s\n", name, err)
		}
		srv.LogEvent("restart", container.ID, srv.daemon.Repositories().ImageName(container.Image))
	} else {
		return job.Errorf("No such container: %s\n", name)
	}
	return engine.StatusOK
}

func (srv *Server) ContainerDestroy(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Not enough arguments. Usage: %s CONTAINER\n", job.Name)
	}
	name := job.Args[0]
	removeVolume := job.GetenvBool("removeVolume")
	removeLink := job.GetenvBool("removeLink")
	stop := job.GetenvBool("stop")
	kill := job.GetenvBool("kill")

	container := srv.daemon.Get(name)

	if removeLink {
		if container == nil {
			return job.Errorf("No such link: %s", name)
		}
		name, err := daemon.GetFullContainerName(name)
		if err != nil {
			job.Error(err)
		}
		parent, n := path.Split(name)
		if parent == "/" {
			return job.Errorf("Conflict, cannot remove the default name of the container")
		}
		pe := srv.daemon.ContainerGraph().Get(parent)
		if pe == nil {
			return job.Errorf("Cannot get parent %s for name %s", parent, name)
		}
		parentContainer := srv.daemon.Get(pe.ID())

		if parentContainer != nil {
			parentContainer.DisableLink(n)
		}

		if err := srv.daemon.ContainerGraph().Delete(name); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
	}

	if container != nil {
		if container.State.IsRunning() {
			if stop {
				if err := container.Stop(5); err != nil {
					return job.Errorf("Could not stop running container, cannot remove - %v", err)
				}
			} else if kill {
				if err := container.Kill(); err != nil {
					return job.Errorf("Could not kill running container, cannot remove - %v", err)
				}
			} else {
				return job.Errorf("You cannot remove a running container. Stop the container before attempting removal or use -s or -k")
			}
		}
		if err := srv.daemon.Destroy(container); err != nil {
			return job.Errorf("Cannot destroy container %s: %s", name, err)
		}
		srv.LogEvent("destroy", container.ID, srv.daemon.Repositories().ImageName(container.Image))

		if removeVolume {
			var (
				volumes     = make(map[string]struct{})
				binds       = make(map[string]struct{})
				usedVolumes = make(map[string]*daemon.Container)
			)

			// the volume id is always the base of the path
			getVolumeId := func(p string) string {
				return filepath.Base(strings.TrimSuffix(p, "/layer"))
			}

			// populate bind map so that they can be skipped and not removed
			for _, bind := range container.HostConfig().Binds {
				source := strings.Split(bind, ":")[0]
				// TODO: refactor all volume stuff, all of it
				// it is very important that we eval the link or comparing the keys to container.Volumes will not work
				//
				// eval symlink can fail, ref #5244 if we receive an is not exist error we can ignore it
				p, err := filepath.EvalSymlinks(source)
				if err != nil && !os.IsNotExist(err) {
					return job.Error(err)
				}
				if p != "" {
					source = p
				}
				binds[source] = struct{}{}
			}

			// Store all the deleted containers volumes
			for _, volumeId := range container.Volumes {
				// Skip the volumes mounted from external
				// bind mounts here will will be evaluated for a symlink
				if _, exists := binds[volumeId]; exists {
					continue
				}

				volumeId = getVolumeId(volumeId)
				volumes[volumeId] = struct{}{}
			}

			// Retrieve all volumes from all remaining containers
			for _, container := range srv.daemon.List() {
				for _, containerVolumeId := range container.Volumes {
					containerVolumeId = getVolumeId(containerVolumeId)
					usedVolumes[containerVolumeId] = container
				}
			}

			for volumeId := range volumes {
				// If the requested volu
				if c, exists := usedVolumes[volumeId]; exists {
					log.Printf("The volume %s is used by the container %s. Impossible to remove it. Skipping.\n", volumeId, c.ID)
					continue
				}
				if err := srv.daemon.Volumes().Delete(volumeId); err != nil {
					return job.Errorf("Error calling volumes.Delete(%q): %v", volumeId, err)
				}
			}
		}
	} else {
		return job.Errorf("No such container: %s", name)
	}
	return engine.StatusOK
}

func (srv *Server) setHostConfig(container *daemon.Container, hostConfig *runconfig.HostConfig) error {
	// Validate the HostConfig binds. Make sure that:
	// the source exists
	for _, bind := range hostConfig.Binds {
		splitBind := strings.Split(bind, ":")
		source := splitBind[0]

		// ensure the source exists on the host
		_, err := os.Stat(source)
		if err != nil && os.IsNotExist(err) {
			err = os.MkdirAll(source, 0755)
			if err != nil {
				return fmt.Errorf("Could not create local directory '%s' for bind mount: %s!", source, err.Error())
			}
		}
	}
	// Register any links from the host config before starting the container
	if err := srv.daemon.RegisterLinks(container, hostConfig); err != nil {
		return err
	}
	container.SetHostConfig(hostConfig)
	container.ToDisk()

	return nil
}

func (srv *Server) ContainerStart(job *engine.Job) engine.Status {
	if len(job.Args) < 1 {
		return job.Errorf("Usage: %s container_id", job.Name)
	}
	var (
		name      = job.Args[0]
		daemon    = srv.daemon
		container = daemon.Get(name)
	)

	if container == nil {
		return job.Errorf("No such container: %s", name)
	}

	if container.State.IsRunning() {
		return job.Errorf("Container already started")
	}

	// If no environment was set, then no hostconfig was passed.
	if len(job.Environ()) > 0 {
		hostConfig := runconfig.ContainerHostConfigFromJob(job)
		if err := srv.setHostConfig(container, hostConfig); err != nil {
			return job.Error(err)
		}
	}
	if err := container.Start(); err != nil {
		return job.Errorf("Cannot start container %s: %s", name, err)
	}
	srv.LogEvent("start", container.ID, daemon.Repositories().ImageName(container.Image))

	return engine.StatusOK
}

func (srv *Server) ContainerStop(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}
	var (
		name = job.Args[0]
		t    = 10
	)
	if job.EnvExists("t") {
		t = job.GetenvInt("t")
	}
	if container := srv.daemon.Get(name); container != nil {
		if !container.State.IsRunning() {
			return job.Errorf("Container already stopped")
		}
		if err := container.Stop(int(t)); err != nil {
			return job.Errorf("Cannot stop container %s: %s\n", name, err)
		}
		srv.LogEvent("stop", container.ID, srv.daemon.Repositories().ImageName(container.Image))
	} else {
		return job.Errorf("No such container: %s\n", name)
	}
	return engine.StatusOK
}

func (srv *Server) ContainerWait(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s", job.Name)
	}
	name := job.Args[0]
	if container := srv.daemon.Get(name); container != nil {
		status, _ := container.State.WaitStop(-1 * time.Second)
		job.Printf("%d\n", status)
		return engine.StatusOK
	}
	return job.Errorf("%s: no such container: %s", job.Name, name)
}

func (srv *Server) ContainerResize(job *engine.Job) engine.Status {
	if len(job.Args) != 3 {
		return job.Errorf("Not enough arguments. Usage: %s CONTAINER HEIGHT WIDTH\n", job.Name)
	}
	name := job.Args[0]
	height, err := strconv.Atoi(job.Args[1])
	if err != nil {
		return job.Error(err)
	}
	width, err := strconv.Atoi(job.Args[2])
	if err != nil {
		return job.Error(err)
	}
	if container := srv.daemon.Get(name); container != nil {
		if err := container.Resize(height, width); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
	}
	return job.Errorf("No such container: %s", name)
}

func (srv *Server) ContainerLogs(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}

	var (
		name   = job.Args[0]
		stdout = job.GetenvBool("stdout")
		stderr = job.GetenvBool("stderr")
		tail   = job.Getenv("tail")
		follow = job.GetenvBool("follow")
		times  = job.GetenvBool("timestamps")
		lines  = -1
		format string
	)
	if !(stdout || stderr) {
		return job.Errorf("You must choose at least one stream")
	}
	if times {
		format = time.RFC3339Nano
	}
	if tail == "" {
		tail = "all"
	}
	container := srv.daemon.Get(name)
	if container == nil {
		return job.Errorf("No such container: %s", name)
	}
	cLog, err := container.ReadLog("json")
	if err != nil && os.IsNotExist(err) {
		// Legacy logs
		utils.Debugf("Old logs format")
		if stdout {
			cLog, err := container.ReadLog("stdout")
			if err != nil {
				utils.Errorf("Error reading logs (stdout): %s", err)
			} else if _, err := io.Copy(job.Stdout, cLog); err != nil {
				utils.Errorf("Error streaming logs (stdout): %s", err)
			}
		}
		if stderr {
			cLog, err := container.ReadLog("stderr")
			if err != nil {
				utils.Errorf("Error reading logs (stderr): %s", err)
			} else if _, err := io.Copy(job.Stderr, cLog); err != nil {
				utils.Errorf("Error streaming logs (stderr): %s", err)
			}
		}
	} else if err != nil {
		utils.Errorf("Error reading logs (json): %s", err)
	} else {
		if tail != "all" {
			var err error
			lines, err = strconv.Atoi(tail)
			if err != nil {
				utils.Errorf("Failed to parse tail %s, error: %v, show all logs", err)
				lines = -1
			}
		}
		if lines != 0 {
			if lines > 0 {
				f := cLog.(*os.File)
				ls, err := tailfile.TailFile(f, lines)
				if err != nil {
					return job.Error(err)
				}
				tmp := bytes.NewBuffer([]byte{})
				for _, l := range ls {
					fmt.Fprintf(tmp, "%s\n", l)
				}
				cLog = tmp
			}
			dec := json.NewDecoder(cLog)
			for {
				l := &utils.JSONLog{}

				if err := dec.Decode(l); err == io.EOF {
					break
				} else if err != nil {
					utils.Errorf("Error streaming logs: %s", err)
					break
				}
				logLine := l.Log
				if times {
					logLine = fmt.Sprintf("%s %s", l.Created.Format(format), logLine)
				}
				if l.Stream == "stdout" && stdout {
					fmt.Fprintf(job.Stdout, "%s", logLine)
				}
				if l.Stream == "stderr" && stderr {
					fmt.Fprintf(job.Stderr, "%s", logLine)
				}
			}
		}
	}
	if follow {
		errors := make(chan error, 2)
		if stdout {
			stdoutPipe := container.StdoutLogPipe()
			go func() {
				errors <- utils.WriteLog(stdoutPipe, job.Stdout, format)
			}()
		}
		if stderr {
			stderrPipe := container.StderrLogPipe()
			go func() {
				errors <- utils.WriteLog(stderrPipe, job.Stderr, format)
			}()
		}
		err := <-errors
		if err != nil {
			utils.Errorf("%s", err)
		}
	}
	return engine.StatusOK
}

func (srv *Server) ContainerAttach(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}

	var (
		name   = job.Args[0]
		logs   = job.GetenvBool("logs")
		stream = job.GetenvBool("stream")
		stdin  = job.GetenvBool("stdin")
		stdout = job.GetenvBool("stdout")
		stderr = job.GetenvBool("stderr")
	)

	container := srv.daemon.Get(name)
	if container == nil {
		return job.Errorf("No such container: %s", name)
	}

	//logs
	if logs {
		cLog, err := container.ReadLog("json")
		if err != nil && os.IsNotExist(err) {
			// Legacy logs
			utils.Debugf("Old logs format")
			if stdout {
				cLog, err := container.ReadLog("stdout")
				if err != nil {
					utils.Errorf("Error reading logs (stdout): %s", err)
				} else if _, err := io.Copy(job.Stdout, cLog); err != nil {
					utils.Errorf("Error streaming logs (stdout): %s", err)
				}
			}
			if stderr {
				cLog, err := container.ReadLog("stderr")
				if err != nil {
					utils.Errorf("Error reading logs (stderr): %s", err)
				} else if _, err := io.Copy(job.Stderr, cLog); err != nil {
					utils.Errorf("Error streaming logs (stderr): %s", err)
				}
			}
		} else if err != nil {
			utils.Errorf("Error reading logs (json): %s", err)
		} else {
			dec := json.NewDecoder(cLog)
			for {
				l := &utils.JSONLog{}

				if err := dec.Decode(l); err == io.EOF {
					break
				} else if err != nil {
					utils.Errorf("Error streaming logs: %s", err)
					break
				}
				if l.Stream == "stdout" && stdout {
					fmt.Fprintf(job.Stdout, "%s", l.Log)
				}
				if l.Stream == "stderr" && stderr {
					fmt.Fprintf(job.Stderr, "%s", l.Log)
				}
			}
		}
	}

	//stream
	if stream {
		var (
			cStdin           io.ReadCloser
			cStdout, cStderr io.Writer
			cStdinCloser     io.Closer
		)

		if stdin {
			r, w := io.Pipe()
			go func() {
				defer w.Close()
				defer utils.Debugf("Closing buffered stdin pipe")
				io.Copy(w, job.Stdin)
			}()
			cStdin = r
			cStdinCloser = job.Stdin
		}
		if stdout {
			cStdout = job.Stdout
		}
		if stderr {
			cStderr = job.Stderr
		}

		<-srv.daemon.Attach(container, cStdin, cStdinCloser, cStdout, cStderr)

		// If we are in stdinonce mode, wait for the process to end
		// otherwise, simply return
		if container.Config.StdinOnce && !container.Config.Tty {
			container.State.WaitStop(-1 * time.Second)
		}
	}
	return engine.StatusOK
}

func (srv *Server) ContainerCopy(job *engine.Job) engine.Status {
	if len(job.Args) != 2 {
		return job.Errorf("Usage: %s CONTAINER RESOURCE\n", job.Name)
	}

	var (
		name     = job.Args[0]
		resource = job.Args[1]
	)

	if container := srv.daemon.Get(name); container != nil {

		data, err := container.Copy(resource)
		if err != nil {
			return job.Error(err)
		}
		defer data.Close()

		if _, err := io.Copy(job.Stdout, data); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
	}
	return job.Errorf("No such container: %s", name)
}
