package daemon

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/graphdb"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/parsers/filters"
)

// List returns an array of all containers registered in the daemon.
func (daemon *Daemon) List() []*Container {
	return daemon.containers.List()
}

func (daemon *Daemon) Containers(job *engine.Job) engine.Status {
	var (
		foundBefore bool
		displayed   int
		all         = job.GetenvBool("all")
		since       = job.Getenv("since")
		before      = job.Getenv("before")
		n           = job.GetenvInt("limit")
		size        = job.GetenvBool("size")
		psFilters   filters.Args
		filt_exited []int
	)
	outs := engine.NewTable("Created", 0)

	psFilters, err := filters.FromParam(job.Getenv("filters"))
	if err != nil {
		return job.Error(err)
	}
	if i, ok := psFilters["exited"]; ok {
		for _, value := range i {
			code, err := strconv.Atoi(value)
			if err != nil {
				return job.Error(err)
			}
			filt_exited = append(filt_exited, code)
		}
	}

	names := map[string][]string{}
	daemon.ContainerGraph().Walk("/", func(p string, e *graphdb.Entity) error {
		names[e.ID()] = append(names[e.ID()], p)
		return nil
	}, -1)

	var beforeCont, sinceCont *Container
	if before != "" {
		beforeCont = daemon.Get(before)
		if beforeCont == nil {
			return job.Error(fmt.Errorf("Could not find container with name or id %s", before))
		}
	}

	if since != "" {
		sinceCont = daemon.Get(since)
		if sinceCont == nil {
			return job.Error(fmt.Errorf("Could not find container with name or id %s", since))
		}
	}

	errLast := errors.New("last container")
	writeCont := func(container *Container) error {
		container.Lock()
		defer container.Unlock()
		if !container.Running && !all && n <= 0 && since == "" && before == "" {
			return nil
		}

		if !psFilters.Match("name", container.Name) {
			return nil
		}

		if !psFilters.Match("id", container.ID) {
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
		if len(filt_exited) > 0 && !container.Running {
			should_skip := true
			for _, code := range filt_exited {
				if code == container.ExitCode {
					should_skip = false
					break
				}
			}
			if should_skip {
				return nil
			}
		}

		if !psFilters.Match("status", container.State.StateString()) {
			return nil
		}
		displayed++
		out := &engine.Env{}
		out.Set("Id", container.ID)
		out.SetList("Names", names[container.ID])
		out.Set("Image", daemon.Repositories().ImageName(container.Image))
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

	for _, container := range daemon.List() {
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
