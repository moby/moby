package daemon

import (
	"os/exec"
	"strconv"
	"strings"

	"github.com/docker/docker/engine"
)

func (daemon *Daemon) ContainerTop(job *engine.Job) engine.Status {
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

	if container := daemon.Get(name); container != nil {
		if !container.IsRunning() {
			return job.Errorf("Container %s is not running", name)
		}
		pids, err := daemon.ExecutionDriver().GetPidsForContainer(container.ID)
		if err != nil {
			return job.Error(err)
		}
		output, err := exec.Command("ps", strings.Split(psArgs, " ")...).Output()
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
