package main

import (
	"github.com/dotcloud/docker/rcli"
	"github.com/dotcloud/docker/fake"
	"github.com/dotcloud/docker/future"
	"bufio"
	"errors"
	"log"
	"io"
	"io/ioutil"
	"os/exec"
	"flag"
	"fmt"
	"github.com/kr/pty"
	"strings"
	"bytes"
	"text/tabwriter"
	"sort"
	"os"
	"time"
	"net/http"
)


func (docker *Docker) Name() string {
	return "docker"
}

func (docker *Docker) Help() string {
	help := "Usage: docker COMMAND [arg...]\n\nA self-sufficient runtime for linux containers.\n\nCommands:\n"
	for _, cmd := range [][]interface{}{
		{"run", "Run a command in a container"},
		{"list", "Display a list of containers"},
		{"pull", "Download a tarball and create a container from it"},
		{"put", "Upload a tarball and create a container from it"},
		{"rm", "Remove containers"},
		{"wait", "Wait for the state of a container to change"},
		{"stop", "Stop a running container"},
		{"logs", "Fetch the logs of a container"},
		{"diff", "Inspect changes on a container's filesystem"},
		{"commit", "Save the state of a container"},
		{"attach", "Attach to the standard inputs and outputs of a running container"},
		{"info", "Display system-wide information"},
		{"web", "Generate a web UI"},
	} {
		help += fmt.Sprintf("    %-10.10s%s\n", cmd...)
	}
	return help
}


func (docker *Docker) CmdList(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := rcli.Subcmd(stdout, "list", "[OPTIONS] [NAME]", "List containers")
	limit := flags.Int("l", 0, "Only show the N most recent versions of each name")
	quiet := flags.Bool("q", false, "only show numeric IDs")
	flags.Parse(args)
	if flags.NArg() > 1 {
		flags.Usage()
		return nil
	}
	var nameFilter string
	if flags.NArg() == 1 {
		nameFilter = flags.Arg(0)
	}
	var names []string
	for name := range docker.containersByName {
		names = append(names, name)
	}
	sort.Strings(names)
	w := tabwriter.NewWriter(stdout, 20, 1, 3, ' ', 0)
	if (!*quiet) {
		fmt.Fprintf(w, "NAME\tID\tCREATED\tSOURCE\tSIZE\tCHANGES\tRUNNING\tCOMMAND\n")
	}
	for _, name := range names {
		if nameFilter != "" && nameFilter != name {
			continue
		}
		for idx, container := range *docker.containersByName[name] {
			if *limit > 0 && idx >= *limit {
				break
			}
			if !*quiet {
				for idx, field := range []string{
					/* NAME */	container.Name,
					/* ID */	container.Id,
					/* CREATED */	future.HumanDuration(time.Now().Sub(container.Created)) + " ago",
					/* SOURCE */	container.Source,
					/* SIZE */	fmt.Sprintf("%.1fM", float32(container.Size) / 1024 / 1024),
					/* CHANGES */	fmt.Sprintf("%.1fM", float32(container.BytesChanged) / 1024 / 1024),
					/* RUNNING */	fmt.Sprintf("%v", container.Running),
					/* COMMAND */	container.CmdString(),
				} {
					if idx == 0 {
						w.Write([]byte(field))
					} else {
						w.Write([]byte("\t" + field))
					}
				}
				w.Write([]byte{'\n'})
			} else {
				stdout.Write([]byte(container.Id + "\n"))
			}
		}
	}
	if (!*quiet) {
		w.Flush()
	}
	return nil
}

func (docker *Docker) findContainer(name string) (*Container, bool) {
	// 1: look for container by ID
	if container, exists := docker.containers[name]; exists {
		return container, true
	}
	// 2: look for a container by name (and pick the most recent)
	if containers, exists := docker.containersByName[name]; exists {
		return (*containers)[0], true
	}
	return nil, false
}


func (docker *Docker) CmdRm(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := rcli.Subcmd(stdout, "rm", "[OPTIONS] CONTAINER", "Remove a container")
	if err := flags.Parse(args); err != nil {
		return nil
	}
	for _, name := range flags.Args() {
		if _, err := docker.rm(name); err != nil {
			fmt.Fprintln(stdout, "Error: " + err.Error())
		}
	}
	return nil
}

func (docker *Docker) CmdPull(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	if len(args) < 1 {
		return errors.New("Not enough arguments")
	}
	resp, err := http.Get(args[0])
	if err != nil {
		return err
	}
	layer, err := docker.layers.AddLayer(resp.Body, stdout)
	if err != nil {
		return err
	}
	docker.addContainer(args[0], "download", 0)
	fmt.Fprintln(stdout, layer.Id())
	return nil
}

func (docker *Docker) CmdPut(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	if len(args) < 1 {
		return errors.New("Not enough arguments")
	}
	fmt.Printf("Adding layer\n")
	layer, err := docker.layers.AddLayer(stdin, stdout)
	if err != nil {
		return err
	}
	docker.addContainer(args[0], "upload", 0)
	fmt.Fprintln(stdout, layer.Id())
	return nil
}


func (docker *Docker) CmdCommit(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := rcli.Subcmd(stdout,
		"fork", "[OPTIONS] CONTAINER [DEST]",
		"Duplicate a container")
	// FIXME "-r" to reset changes in the new container
	if err := flags.Parse(args); err != nil {
		return nil
	}
	srcName, dstName := flags.Arg(0), flags.Arg(1)
	if srcName == "" {
		flags.Usage()
		return nil
	}
	if dstName == "" {
		dstName = srcName
	}
	if src, exists := docker.findContainer(srcName); exists {
		dst := docker.addContainer(dstName, "snapshot:" + src.Id, src.Size)
		fmt.Fprintln(stdout, dst.Id)
		return nil
	}
	return errors.New("No such container: " + srcName)
}

func (docker *Docker) CmdTar(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := rcli.Subcmd(stdout,
		"tar", "CONTAINER",
		"Stream the contents of a container as a tar archive")
	if err := flags.Parse(args); err != nil {
		return nil
	}
	name := flags.Arg(0)
	if _, exists := docker.findContainer(name); exists {
		// Stream the entire contents of the container (basically a volatile snapshot)
		return fake.WriteFakeTar(stdout)
	}
	return errors.New("No such container: " + name)
}

func (docker *Docker) CmdDiff(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := rcli.Subcmd(stdout,
		"diff", "CONTAINER [OPTIONS]",
		"Inspect changes on a container's filesystem")
	fl_diff := flags.Bool("d", true, "Show changes in diff format")
	fl_bytes := flags.Bool("b", false, "Show how many bytes have been changed")
	fl_list := flags.Bool("l", false, "Show a list of changed files")
	if err := flags.Parse(args); err != nil {
		return nil
	}
	if flags.NArg() < 1 {
		return errors.New("Not enough arguments")
	}
	if container, exists := docker.findContainer(flags.Arg(0)); !exists {
		return errors.New("No such container")
	} else if *fl_bytes {
		fmt.Fprintf(stdout, "%d\n", container.BytesChanged)
	} else if *fl_list {
		// FAKE
		fmt.Fprintf(stdout, strings.Join([]string{
			"/etc/postgres/pg.conf",
			"/etc/passwd",
			"/var/lib/postgres",
			"/usr/bin/postgres",
			"/usr/bin/psql",
			"/var/log/postgres",
			"/var/log/postgres/postgres.log",
			"/var/log/postgres/postgres.log.0",
			"/var/log/postgres/postgres.log.1.gz"}, "\n"))
	} else if *fl_diff {
		// Achievement unlocked: embed a diff of your code as a string in your code
		fmt.Fprintf(stdout, `
diff --git a/dockerd/dockerd.go b/dockerd/dockerd.go
index 2dae694..e43caca 100644
--- a/dockerd/dockerd.go
+++ b/dockerd/dockerd.go
@@ -158,6 +158,7 @@ func (docker *Docker) CmdDiff(stdin io.ReadCloser, stdout io.Writer, args ...str
        flags := rcli.Subcmd(stdout,
                "diff", "CONTAINER [OPTIONS]",
                "Inspect changes on a container's filesystem")
+       fl_diff := flags.Bool("d", true, "Show changes in diff format")
        fl_bytes := flags.Bool("b", false, "Show how many bytes have been changes")
        fl_list := flags.Bool("l", false, "Show a list of changed files")
        fl_download := flags.Bool("d", false, "Download the changes as gzipped tar stream")
`)
		return nil
	} else {
		flags.Usage()
		return nil
	}
	return nil
}


// ByDate wraps an array of layers so they can be sorted by date (most recent first)

type ByDate []*Container

func (c *ByDate) Len() int {
	return len(*c)
}

func (c *ByDate) Less(i, j int) bool {
	containers := *c
	return containers[j].Created.Before(containers[i].Created)
}

func (c *ByDate) Swap(i, j int) {
	containers := *c
	tmp := containers[i]
	containers[i] = containers[j]
	containers[j] = tmp
}

func (c *ByDate) Add(container *Container) {
	*c = append(*c, container)
	sort.Sort(c)
}

func (c *ByDate) Del(id string) {
	for idx, container := range *c {
		if container.Id == id {
			*c = append((*c)[:idx], (*c)[idx + 1:]...)
		}
	}
}


func (docker *Docker) addContainer(name string, source string, size uint) *Container {
	if size == 0 {
		size = fake.RandomContainerSize()
	}
	c := &Container{
		Id:		future.RandomId(),
		Name:		name,
		Created:	time.Now(),
		Source:		source,
		Size:		size,
		stdinLog: new(bytes.Buffer),
		stdoutLog: new(bytes.Buffer),
	}
	docker.containers[c.Id] = c
	if _, exists := docker.containersByName[c.Name]; !exists {
		docker.containersByName[c.Name] = new(ByDate)
	}
	docker.containersByName[c.Name].Add(c)
	return c

}


func (docker *Docker) rm(id string) (*Container, error) {
	if container, exists := docker.containers[id]; exists {
		if container.Running {
			return nil, errors.New("Container is running: " + id)
		} else {
			// Remove from name lookup
			docker.containersByName[container.Name].Del(container.Id)
			// Remove from id lookup
			delete(docker.containers, container.Id)
			return container, nil
		}
	}
	return nil, errors.New(fmt.Sprintf("No such container: %s", id))
}


func (docker *Docker) CmdLogs(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := rcli.Subcmd(stdout, "logs", "[OPTIONS] CONTAINER", "Fetch the logs of a container")
	if err := flags.Parse(args); err != nil {
		return nil
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return nil
	}
	name := flags.Arg(0)
	if container, exists := docker.findContainer(name); exists {
		if _, err := io.Copy(stdout, container.StdoutLog()); err != nil {
			return err
		}
		return nil
	}
	return errors.New("No such container: " + flags.Arg(0))
}

func (docker *Docker) CmdRun(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := rcli.Subcmd(stdout, "run", "[OPTIONS] CONTAINER COMMAND [ARG...]", "Run a command in a container")
	fl_attach := flags.Bool("a", false, "Attach stdin and stdout")
	fl_tty := flags.Bool("t", false, "Allocate a pseudo-tty")
	if err := flags.Parse(args); err != nil {
		return nil
	}
	if flags.NArg() < 2 {
		flags.Usage()
		return nil
	}
	name, cmd := flags.Arg(0), flags.Args()[1:]
	if container, exists := docker.findContainer(name); exists {
		if container.Running {
			return errors.New("Already running: " + name)
		}
		if *fl_attach {
			return container.Run(cmd[0], cmd[1:], stdin, stdout, *fl_tty)
		} else {
			go container.Run(cmd[0], cmd[1:], ioutil.NopCloser(new(bytes.Buffer)), ioutil.Discard, *fl_tty)
			fmt.Fprintln(stdout, container.Id)
			return nil
		}
	}
	return errors.New("No such container: " + name)
}

func startCommand(cmd *exec.Cmd, interactive bool) (io.WriteCloser, io.ReadCloser, error) {
	if interactive {
		term, err := pty.Start(cmd)
		if err != nil {
			return nil, nil, err
		}
		return term, term, nil
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return stdin, stdout, nil
}


func main() {
	future.Seed()
	flag.Parse()
	docker, err := New()
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		if err := rcli.ListenAndServeHTTP(":8080", docker); err != nil {
			log.Fatal(err)
		}
	}()
	if err := rcli.ListenAndServeTCP(":4242", docker); err != nil {
		log.Fatal(err)
	}
}

func New() (*Docker, error) {
	store, err := future.NewStore("/var/lib/docker/layers")
	if err != nil {
		return nil, err
	}
	if err := store.Init(); err != nil {
		return nil, err
	}
	return &Docker{
		containersByName: make(map[string]*ByDate),
		containers: make(map[string]*Container),
		layers: store,
	}, nil
}


func (docker *Docker) CmdMirror(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	_, err := io.Copy(stdout, stdin)
	return err
}

func (docker *Docker) CmdDebug(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	for {
		if line, err := bufio.NewReader(stdin).ReadString('\n'); err == nil {
			fmt.Printf("--- %s", line)
		} else if err == io.EOF {
			if len(line) > 0 {
				fmt.Printf("--- %s\n", line)
			}
			break
		} else {
			return err
		}
	}
	return nil
}

func (docker *Docker) CmdWeb(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := rcli.Subcmd(stdout, "web", "[OPTIONS]", "A web UI for docker")
	showurl := flags.Bool("u", false, "Return the URL of the web UI")
	if err := flags.Parse(args); err != nil {
		return nil
	}
	if *showurl {
		fmt.Fprintln(stdout, "http://localhost:4242/web")
	} else {
		if file, err := os.Open("dockerweb.html"); err != nil {
			return err
		} else if _, err := io.Copy(stdout, file); err != nil {
			return err
		}
	}
	return nil
}


type Docker struct {
	containers		map[string]*Container
	containersByName	map[string]*ByDate
	layers			*future.Store
}

type Container struct {
	Id	string
	Name	string
	Created	time.Time
	Source	string
	Size	uint
	FilesChanged uint
	BytesChanged uint
	Running	bool
	Cmd	string
	Args	[]string
	stdoutLog *bytes.Buffer
	stdinLog *bytes.Buffer
}

func (c *Container) Run(command string, args []string, stdin io.ReadCloser, stdout io.Writer, tty bool) error {
	// Not thread-safe
	if c.Running {
		return errors.New("Already running")
	}
	c.Cmd = command
	c.Args = args
	// Reset logs
	c.stdoutLog.Reset()
	c.stdinLog.Reset()
	cmd := exec.Command(c.Cmd, c.Args...)
	cmd_stdin, cmd_stdout, err := startCommand(cmd, tty)
	if err != nil {
		return err
	}
	c.Running = true
	// ADD FAKE RANDOM CHANGES
	c.FilesChanged = fake.RandomFilesChanged()
	c.BytesChanged = fake.RandomBytesChanged()
	copy_out := future.Go(func() error {
		_, err := io.Copy(io.MultiWriter(stdout, c.stdoutLog), cmd_stdout)
		return err
	})
	future.Go(func() error {
		_, err := io.Copy(io.MultiWriter(cmd_stdin, c.stdinLog), stdin)
		cmd_stdin.Close()
		stdin.Close()
		return err
	})
	wait := future.Go(func() error {
		err := cmd.Wait()
		c.Running = false
		return err
	})
	if err := <-copy_out; err != nil {
		if c.Running {
			return err
		}
	}
	if err := <-wait; err != nil {
		if status, ok := err.(*exec.ExitError); ok {
			fmt.Fprintln(stdout, status)
			return nil
		}
		return err
	}
	return nil
}

func (c *Container) StdoutLog() io.Reader {
	return strings.NewReader(c.stdoutLog.String())
}

func (c *Container) StdinLog() io.Reader {
	return strings.NewReader(c.stdinLog.String())
}

func (c *Container) CmdString() string {
	return strings.Join(append([]string{c.Cmd}, c.Args...), " ")
}

