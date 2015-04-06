package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/graph"
	"github.com/docker/docker/nat"
	"github.com/docker/docker/pkg/graphdb"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/docker/docker/utils"
)

// List returns an array of all containers registered in the daemon.
func (daemon *Daemon) List() []*Container {
	return daemon.containers.List()
}

type ByCreated []types.Container

func (r ByCreated) Len() int           { return len(r) }
func (r ByCreated) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r ByCreated) Less(i, j int) bool { return r[i].Created < r[j].Created }

func (daemon *Daemon) Containers(job *engine.Job) error {
	var (
		foundBefore bool
		displayed   int
		all         = job.GetenvBool("all")
		since       = job.Getenv("since")
		before      = job.Getenv("before")
		n           = job.GetenvInt("limit")
		size        = job.GetenvBool("size")
		psFilters   filters.Args
		filtExited  []int
	)
	containers := []types.Container{}

	psFilters, err := filters.FromParam(job.Getenv("filters"))
	if err != nil {
		return err
	}
	if i, ok := psFilters["exited"]; ok {
		for _, value := range i {
			code, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			filtExited = append(filtExited, code)
		}
	}

	if i, ok := psFilters["status"]; ok {
		for _, value := range i {
			if value == "exited" {
				all = true
			}
		}
	}
	names := map[string][]string{}
	daemon.ContainerGraph().Walk("/", func(p string, e *graphdb.Entity) error {
		names[e.ID()] = append(names[e.ID()], p)
		return nil
	}, 1)

	var beforeCont, sinceCont *Container
	if before != "" {
		beforeCont, err = daemon.Get(before)
		if err != nil {
			return err
		}
	}

	if since != "" {
		sinceCont, err = daemon.Get(since)
		if err != nil {
			return err
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

		if !psFilters.MatchKVList("label", container.Config.Labels) {
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
		if len(filtExited) > 0 {
			shouldSkip := true
			for _, code := range filtExited {
				if code == container.ExitCode && !container.Running {
					shouldSkip = false
					break
				}
			}
			if shouldSkip {
				return nil
			}
		}

		if !psFilters.Match("status", container.State.StateString()) {
			return nil
		}
		displayed++
		newC := types.Container{
			ID:    container.ID,
			Names: names[container.ID],
		}
		img := container.Config.Image
		_, tag := parsers.ParseRepositoryTag(container.Config.Image)
		if tag == "" {
			img = utils.ImageReference(img, graph.DEFAULTTAG)
		}
		newC.Image = img
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

			newC.Command = fmt.Sprintf("%s %s", container.Path, argsAsString)
		} else {
			newC.Command = fmt.Sprintf("%s", container.Path)
		}
		newC.Created = int(container.Created.Unix())
		newC.Status = container.State.String()

		newC.Ports = []types.Port{}
		for port, bindings := range container.NetworkSettings.Ports {
			p, _ := nat.ParsePort(port.Port())
			if len(bindings) == 0 {
				newC.Ports = append(newC.Ports, types.Port{
					PrivatePort: p,
					Type:        port.Proto(),
				})
				continue
			}
			for _, binding := range bindings {
				h, _ := nat.ParsePort(binding.HostPort)
				newC.Ports = append(newC.Ports, types.Port{
					PrivatePort: p,
					PublicPort:  h,
					Type:        port.Proto(),
					IP:          binding.HostIp,
				})
			}
		}

		if size {
			sizeRw, sizeRootFs := container.GetSize()
			newC.SizeRw = int(sizeRw)
			newC.SizeRootFs = int(sizeRootFs)
		}
		newC.Labels = container.Config.Labels
		containers = append(containers, newC)
		return nil
	}

	for _, container := range daemon.List() {
		if err := writeCont(container); err != nil {
			if err != errLast {
				return err
			}
			break
		}
	}
	sort.Sort(sort.Reverse(ByCreated(containers)))
	if err = json.NewEncoder(job.Stdout).Encode(containers); err != nil {
		return err
	}
	return nil
}
