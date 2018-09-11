//+build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

// fieldsASCII is similar to strings.Fields but only allows ASCII whitespaces
func fieldsASCII(s string) []string {
	fn := func(r rune) bool {
		switch r {
		case '\t', '\n', '\f', '\r', ' ':
			return true
		}
		return false
	}
	return strings.FieldsFunc(s, fn)
}

func appendProcess2ProcList(procList *container.ContainerTopOKBody, fields []string) {
	// Make sure number of fields equals number of header titles
	// merging "overhanging" fields
	process := fields[:len(procList.Titles)-1]
	process = append(process, strings.Join(fields[len(procList.Titles)-1:], " "))
	procList.Processes = append(procList.Processes, process)
}

func hasPid(procs []uint32, pid int) bool {
	for _, p := range procs {
		if int(p) == pid {
			return true
		}
	}
	return false
}

func parsePSOutput(output []byte, procs []uint32, addPid bool) (*container.ContainerTopOKBody, error) {
	procList := &container.ContainerTopOKBody{}

	lines := strings.Split(string(output), "\n")
	procList.Titles = fieldsASCII(lines[0])

	errorNoPid := errors.New("Couldn't find PID field in ps output")

	var pidIndex, firstCol int
	if addPid { // Option "-o pid" was prepended to ps args, first field is PID
		// validate it is there
		if len(procList.Titles) < 1 || procList.Titles[0] != "PID" {
			return nil, errorNoPid
		}
		pidIndex = 0 // PID is in first column
		firstCol = 1 // filter out the first column
		// remove the first column
		procList.Titles = procList.Titles[firstCol:]
	} else {
		// find the PID column
		pidIndex = -1
		for i, name := range procList.Titles {
			if name == "PID" {
				pidIndex = i
				break
			}
		}
		if pidIndex == -1 {
			return nil, errorNoPid
		}
	}

	// loop through the output and extract the PID from each line
	// fixing #30580, be able to display thread line also when "m" option used
	// in "docker top" client command
	preContainedPidFlag := false
	for _, line := range lines[1:] {
		if len(line) == 0 {
			continue
		}
		fields := fieldsASCII(line)

		var (
			p   int
			err error
		)

		if fields[pidIndex] == "-" {
			if preContainedPidFlag {
				appendProcess2ProcList(procList, fields[firstCol:])
			}
			continue
		}
		p, err = strconv.Atoi(fields[pidIndex])
		if err != nil {
			return nil, fmt.Errorf("Unexpected pid '%s': %s", fields[pidIndex], err)
		}

		if hasPid(procs, p) {
			preContainedPidFlag = true
			appendProcess2ProcList(procList, fields[firstCol:])
			continue
		}
		preContainedPidFlag = false
	}
	return procList, nil
}

// psPidsArg converts a slice of PIDs to a string consisting
// of comma-separated list of PIDs prepended by "-q".
// For example, psPidsArg([]uint32{1,2,3}) returns "-q1,2,3".
func psPidsArg(pids []uint32) string {
	b := []byte{'-', 'q'}
	for i, p := range pids {
		b = strconv.AppendUint(b, uint64(p), 10)
		if i < len(pids)-1 {
			b = append(b, ',')
		}
	}
	return string(b)
}

// customFields checks whether o/-o/--format <field,...> option
// was given to ps
func customFields(args []string) bool {
	/*
	 * ps allows for a few fancy ways to provide an argument:
	 *
	 *	ps --format cmd
	 *	ps --format=cmd
	 *	ps -o cmd
	 *	ps -ocmd
	 *	ps ocmd
	 *
	 * The above five ways are equivalent, with cmd as an argument.
	 *
	 * In addition, any single character option can be mixed
	 * together with others (as long as an option with an argument
	 * is the last one):
	 *
	 *	ps efocmd # here o is option, cmd is argument
	 *
	 * So, we also need to make sure we don't recognize option
	 * arguments as options. Here are a few examples in which
	 * ocmd is NOT an option:
	 *
	 *	ps -C ocmd
	 *	ps eUocmd
	 *
	 * Due to all this, this check is not so trivial.
	 */

	// parse an argument, figure out if
	//  - the following argument is an option value (skip)
	//  - custom format option is specified (found)
	var skip, found bool
	check := func(opt string) {
		if len(opt) == 0 {
			return
		}

		// There are three types of ps options that can have an argument
		// 1. Long GNU style options, like --pid 1 or --pid=1.
		longOpt := len(opt) > 2 && opt[0] == '-' && opt[1] == '-'
		if longOpt {
			// it can either be --opt or --opt=arg
			optArg := strings.Split(opt, "=")
			switch optArg[0] {
			case "--format":
				found = true
				return
			case
				"--cols",
				"--columns",
				"--Group",
				"--group",
				"--lines",
				"--pid",
				"--ppid",
				"--quick-pid",
				"--rows",
				"--sid",
				"--sort",
				"--tty",
				"--User",
				"--user",
				"--width":
				if len(optArg) == 1 { // --opt arg
					skip = true
				}
				// --opt=arg
				return
			}
		}

		// 2. Short options, like -q.
		if len(opt) >= 2 && opt[0] == '-' {
			switch opt[:2] {
			case "-o":
				found = true
				return
			case "-C", "-G", "-g", "-O", "-p",
				"-q", "-s", "-t", "-u", "-U":
				if len(opt) == 2 {
					skip = true
				}
				// argument is right here, like -Uroot
				return
			}
		}

		// 3. Single character options with no dash, which can be
		// in the middle of the opt, like eq1.
		for i, c := range opt {
			switch c {
			case 'o':
				found = true
				return
			case 'p', 'q', 't', 'U', 'O', 'k':
				if i+1 == len(opt) { // last character
					skip = true
				}
				// the rest is option argument
				return
			}
		}
	}

	for _, arg := range args {
		if skip { // arg is an option argument (like -U root)
			skip = false
			continue
		}
		// shortcut, check for trivial cases: separate o / -o / --format
		if strings.HasPrefix(arg, "-o") || strings.HasPrefix(arg, "o") || strings.HasPrefix(arg, "--format") {
			return true
		}

		// complicated cases, o can be in the middle,
		// plus we need to skip option arguments
		check(arg)
		if found {
			return true
		}
	}

	return false
}

// ContainerTop lists the processes running inside of the given
// container by calling ps with the given args, or with the flags
// "-ef" if no args are given.  An error is returned if the container
// is not found, or is not running, or if there are any problems
// running ps, or parsing the output.
func (daemon *Daemon) ContainerTop(name string, psArgs string) (*container.ContainerTopOKBody, error) {
	customArgs := true
	if psArgs == "" {
		customArgs = false
		psArgs = "-ef"
	}

	container, err := daemon.GetContainer(name)
	if err != nil {
		return nil, err
	}

	if !container.IsRunning() {
		return nil, errNotRunning(container.ID)
	}

	if container.IsRestarting() {
		return nil, errContainerIsRestarting(container.ID)
	}

	procs, err := daemon.containerd.ListPids(context.Background(), container.ID)
	if err != nil {
		return nil, err
	}

	args := strings.Split(psArgs, " ")

	var extraPidColumn bool
	if customArgs && customFields(args) {
		// make sure the PID field is shown in the first column
		args = append([]string{"-opid"}, args...)
		extraPidColumn = true
	}

	pids := psPidsArg(procs)
	output, err := exec.Command("ps", append(args, pids)...).Output()
	if err != nil {
		// some ps options (such as f, -C) can't be used
		// together with q, so retry without it, listing
		// all the processes and applying a filter.
		output, err = exec.Command("ps", args...).Output()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				// first line of stderr shows why ps failed
				line := bytes.SplitN(ee.Stderr, []byte{'\n'}, 2)
				if len(line) > 0 && len(line[0]) > 0 {
					err = errors.New(string(line[0]))
				}
			}
			return nil, errdefs.System(errors.Wrap(err, "ps"))
		}
	}
	procList, err := parsePSOutput(output, procs, extraPidColumn)
	if err != nil {
		return nil, err
	}
	daemon.LogContainerEvent(container, "top")
	return procList, nil
}
