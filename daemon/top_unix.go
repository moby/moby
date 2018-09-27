//+build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

func validatePSArgs(psArgs string) error {
	// NOTE: \\s does not detect unicode whitespaces.
	// So we use fieldsASCII instead of strings.Fields in parsePSOutput.
	// See https://github.com/docker/docker/pull/24358
	// nolint: gosimple
	re := regexp.MustCompile("\\s+([^\\s]*)=\\s*(PID[^\\s]*)")
	for _, group := range re.FindAllStringSubmatch(psArgs, -1) {
		if len(group) >= 3 {
			k := group[1]
			v := group[2]
			if k != "pid" {
				return fmt.Errorf("specifying \"%s=%s\" is not allowed", k, v)
			}
		}
	}
	return nil
}

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

// parsePSOutput is to parse ps's output
// fieldsASCII for titles is fine, but for content is bad. For example:
// ps -ocmd,ppid,pid
// "/bin/bash /sto 12368" 12222 12399
// if 12368 just equals another container process' pid, there will be error.
// this is the fault of fieldsASCII, so we need to improve.
// There is real result for `ps -e -ocmd,ppid,pid,wchan:14,args`
// ******************* example *********************
// root@dockerdemo:~# ps -e -ocmd,ppid,pid,wchan:14,args
// CMD                          PPID   PID WCHAN          COMMAND
// /root/bin/containerd            1  2761 futex_wait_que /root/bin/containerd
// /lib/systemd/systemd --user     1  3402 ep_poll        /lib/systemd/systemd --user
// (sd-pam)                     3402  3407 sigtimedwait   (sd-pam)
// top                             1  4803 refrigerator   top
// /bin/bash /st 28808             1 15468 wait           /bin/bash /st 28808
// redis-server *:28808        15468 15497 ep_poll        redis-server *:28808
// [kworker/u4:0]                  2 22787 worker_thread  [kworker/u4:0]
// [kworker/1:1]                   2 23039 worker_thread  [kworker/1:1]
// [kworker/0:3]                   2 23445 worker_thread  [kworker/0:3]
// /bin/bash /st 28808             1 24366 wait           /bin/bash /st 28808
// redis-server *:28808        24366 24420 ep_poll        redis-server *:28808
// sshd: root@pts/0              954 25044 poll_schedule_ sshd: root@pts/0
// -bash                       25044 25062 wait_woken     -bash
// [kworker/0:0]                   2 25199 worker_thread  [kworker/0:0]
// [kworker/1:2]                   2 26476 worker_thread  [kworker/1:2]
// /usr/bin/dockerd -H tcp://0     1 28845 futex_wait_que /usr/bin/dockerd -H tcp://0.0.0.0:8410 -H unix:///var/run/docker.sock
// docker-containerd --config  28845 28854 futex_wait_que docker-containerd --config /var/run/docker/containerd/containerd.toml --log-level info
// docker-containerd-shim -nam 28854 29054 futex_wait_que docker-containerd-shim -namespace moby -workdir /var/lib/docker/containerd/daemon/io.contai...
// /bin/bash /sto 6379         29054 29071 wait           /bin/bash /sto 6379
// redis-server *:6379         29071 29120 ep_poll        redis-server *:6379
// sleep 100000                    1 29383 hrtimer_nanosl sleep 100000
// /bin/sh /ocmd                   1 29402 wait           /bin/sh /ocmd
// sleep 100000                29402 29403 hrtimer_nanosl sleep 100000
// /bin/sh /-o                     1 29577 wait           /bin/sh /-o
// sleep 100000                29577 29578 hrtimer_nanosl sleep 100000
// sshd: root@pts/1              954 29622 poll_schedule_ sshd: root@pts/1
// -bash                       29622 29640 wait_woken     -bash
// [kworker/u4:2]                  2 29676 worker_thread  [kworker/u4:2]
// /bin/bash /-o                   1 29691 wait           /bin/bash /-o
// sleep 100000                29691 29692 hrtimer_nanosl sleep 100000
// sshd: root@pts/3              954 30173 -              sshd: root@pts/3
// -bash                       30173 30196 wait           -bash
// [kworker/u4:1]                  2 30211 -              [kworker/u4:1]
// ps -e -ocmd,ppid,pid,wchan: 30196 30239 -              ps -e -ocmd,ppid,pid,wchan:14,args
// /usr/bin/pouchd                 1 31522 futex_wait_que /usr/bin/pouchd
// ****************** algorithm comment ****************
// There are 5 columns: column[0] column[1] column[2] column[3] column[4]
// There is at least one whitespace before each column whose index is large than 0
// So, we can find the vertical dividing lines of the output content which is a two-dimensional matrix.
// The result is just like this:
// CMD                        |  PPID|   PID| WCHAN         | COMMAND
// /root/bin/containerd       |     1|  2761| futex_wait_que| /root/bin/containerd
// /lib/systemd/systemd --user|     1|  3402| ep_poll       | /lib/systemd/systemd --user
// (sd-pam)                   |  3402|  3407| sigtimedwait  | (sd-pam)
// top                        |     1|  4803| refrigerator  | top
// /bin/bash /st 28808        |     1| 15468| wait          | /bin/bash /st 28808
// redis-server *:28808       | 15468| 15497| ep_poll       | redis-server *:28808
// [kworker/u4:0]             |     2| 22787| worker_thread | [kworker/u4:0]
// [kworker/1:1]              |     2| 23039| worker_thread | [kworker/1:1]
// [kworker/0:3]              |     2| 23445| worker_thread | [kworker/0:3]
// /bin/bash /st 28808        |     1| 24366| wait          | /bin/bash /st 28808
// redis-server *:28808       | 24366| 24420| ep_poll       | redis-server *:28808
// sshd: root@pts/0           |   954| 25044| poll_schedule_| sshd: root@pts/0
// -bash                      | 25044| 25062| wait_woken    | -bash
// [kworker/0:0]              |     2| 25199| worker_thread | [kworker/0:0]
// [kworker/1:2]              |     2| 26476| worker_thread | [kworker/1:2]
// /usr/bin/dockerd -H tcp://0|     1| 28845| futex_wait_que| /usr/bin/dockerd -H tcp://0.0.0.0:8410 -H unix:///var/run/docker.sock
// docker-containerd --config | 28845| 28854| futex_wait_que| docker-containerd --config /var/run/docker/containerd/containerd.toml --log-level info
// docker-containerd-shim -nam| 28854| 29054| futex_wait_que| docker-containerd-shim -namespace moby -workdir /var/lib/docker/containerd/daemon/io.contai...
// /bin/bash /sto 6379        | 29054| 29071| wait          | /bin/bash /sto 6379
// redis-server *:6379        | 29071| 29120| ep_poll       | redis-server *:6379
// sleep 100000               |     1| 29383| hrtimer_nanosl| sleep 100000
// /bin/sh /ocmd              |     1| 29402| wait          | /bin/sh /ocmd
// sleep 100000               | 29402| 29403| hrtimer_nanosl| sleep 100000
// /bin/sh /-o                |     1| 29577| wait          | /bin/sh /-o
// sleep 100000               | 29577| 29578| hrtimer_nanosl| sleep 100000
// sshd: root@pts/1           |   954| 29622| poll_schedule_| sshd: root@pts/1
// -bash                      | 29622| 29640| wait_woken    | -bash
// [kworker/u4:2]             |     2| 29676| worker_thread | [kworker/u4:2]
// /bin/bash /-o              |     1| 29691| wait          | /bin/bash /-o
// sleep 100000               | 29691| 29692| hrtimer_nanosl| sleep 100000
// sshd: root@pts/3           |   954| 30173| -             | sshd: root@pts/3
// -bash                      | 30173| 30196| wait          | -bash
// [kworker/u4:1]             |     2| 30211| -             | [kworker/u4:1]
// ps -e -ocmd,ppid,pid,wchan:| 30196| 30239| -             | ps -e -ocmd,ppid,pid,wchan:14,args
// /usr/bin/pouchd            |     1| 31522| futex_wait_que| /usr/bin/pouchd
func parsePSOutput(output []byte, procs []uint32) (*container.ContainerTopOKBody, error) {
	procList := &container.ContainerTopOKBody{}

	lines := strings.Split(string(output), "\n")
	procList.Titles = fieldsASCII(lines[0])

	pidIndex := -1
	for i, name := range procList.Titles {
		if name == "PID" {
			pidIndex = i
			break
		}
	}
	if pidIndex == -1 {
		return nil, fmt.Errorf("Couldn't find PID field in ps output")
	}

	// find the vertical dividing lines of the output content which is a two-dimensional matrix.
	reg := regexp.MustCompile(`[\S]+`)
	idxs := reg.FindAllStringIndex(lines[0], -1)
	idxs[0][0] = 0
	for i, idx := range idxs[1:] {
		start := idx[0] - 1
	Loop2:
		for {
			blNotFind := false
		Loop3:
			for _, line := range lines[1:] {
				if len(line) > start && !unicode.IsSpace(rune(line[start])) {
					blNotFind = true
					start = start - 1
					break Loop3
				}
			}
			if start < idxs[i][1] {
				// insure Loop2 can break if there is some errors
				break Loop2
			}
			if !blNotFind {
				idx[0] = start
				break Loop2
			}
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
		var fields []string
		for i, idx := range idxs {
			var end int
			if i < len(idxs)-1 {
				end = idxs[i+1][0]
			} else {
				// the last column
				end = len(line)
			}
			fields = append(fields, strings.TrimSpace(line[idx[0]:end]))
		}

		var (
			p   int
			err error
		)

		if fields[pidIndex] == "-" {
			if preContainedPidFlag {
				appendProcess2ProcList(procList, fields)
			}
			continue
		}
		p, err = strconv.Atoi(fields[pidIndex])
		if err != nil {
			return nil, fmt.Errorf("Unexpected pid '%s': %s", fields[pidIndex], err)
		}

		if hasPid(procs, p) {
			preContainedPidFlag = true
			appendProcess2ProcList(procList, fields)
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

// ContainerTop lists the processes running inside of the given
// container by calling ps with the given args, or with the flags
// "-ef" if no args are given.  An error is returned if the container
// is not found, or is not running, or if there are any problems
// running ps, or parsing the output.
func (daemon *Daemon) ContainerTop(name string, psArgs string) (*container.ContainerTopOKBody, error) {
	if psArgs == "" {
		psArgs = "-ef"
	}

	if err := validatePSArgs(psArgs); err != nil {
		return nil, err
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
	// can't work without headers
	for _, arg := range args {
		switch arg {
		case
			"--no-header",
			"--no-headers",
			"--no-heading",
			"--no-headings",
			"--noheader",
			"--noheaders",
			"--noheading",
			"--noheadings":
			return nil, errdefs.System(errors.New("option " + arg + " is not allowed"))
		}
	}
	pids := psPidsArg(procs)
	output, err := exec.Command("ps", append(args, pids)...).Output()
	if err != nil {
		// some ps options (such as f) can't be used together with q,
		// so retry without it
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
	procList, err := parsePSOutput(output, procs)
	if err != nil {
		return nil, err
	}
	daemon.LogContainerEvent(container, "top")
	return procList, nil
}
