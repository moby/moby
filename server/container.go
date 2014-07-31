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
	"time"

	"github.com/docker/docker/daemon"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/graphdb"
	"github.com/docker/docker/pkg/tailfile"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

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
