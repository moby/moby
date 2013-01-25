package main

import (
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/rcli"
	"github.com/dotcloud/docker/fake"
	"github.com/dotcloud/docker/future"
	"bufio"
	"errors"
	"log"
	"io"
	"io/ioutil"
	"flag"
	"fmt"
	"strings"
	"bytes"
	"text/tabwriter"
	"sort"
	"os"
	"time"
	"net/http"
)


func (srv *Server) Name() string {
	return "docker"
}

func (srv *Server) Help() string {
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


func (srv *Server) CmdList(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
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
	for name := range srv.containersByName {
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
		for idx, container := range *srv.containersByName[name] {
			if *limit > 0 && idx >= *limit {
				break
			}
			if !*quiet {
				for idx, field := range []string{
					/* NAME */	container.GetUserData("name"),
					/* ID */	container.Id,
					/* CREATED */	future.HumanDuration(time.Now().Sub(container.Created)) + " ago",
					/* SOURCE */	container.GetUserData("source"),
					/* SIZE */	fmt.Sprintf("%.1fM", float32(fake.RandomContainerSize()) / 1024 / 1024),
					/* CHANGES */	fmt.Sprintf("%.1fM", float32(fake.RandomBytesChanged() / 1024 / 1024)),
					/* RUNNING */	fmt.Sprintf("%v", fake.ContainerRunning()),
					/* COMMAND */	fmt.Sprintf("%s %s", container.Path, strings.Join(container.Args, " ")),
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

func (srv *Server) findContainer(name string) (*fake.Container, bool) {
	// 1: look for container by ID
	if container := srv.docker.Get(name); container != nil {
		return fake.NewContainer(container), true
	}
	// 2: look for a container by name (and pick the most recent)
	if containers, exists := srv.containersByName[name]; exists {
		return fake.NewContainer((*containers)[0]), true
	}
	return nil, false
}


func (srv *Server) CmdRm(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := rcli.Subcmd(stdout, "rm", "[OPTIONS] CONTAINER", "Remove a container")
	if err := flags.Parse(args); err != nil {
		return nil
	}
	for _, name := range flags.Args() {
		if _, err := srv.rm(name); err != nil {
			fmt.Fprintln(stdout, "Error: " + err.Error())
		}
	}
	return nil
}

func (srv *Server) CmdPull(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	if len(args) < 1 {
		return errors.New("Not enough arguments")
	}
	resp, err := http.Get(args[0])
	if err != nil {
		return err
	}
	layer, err := srv.layers.AddLayer(resp.Body, stdout)
	if err != nil {
		return err
	}
	container, err := srv.addContainer(layer.Id(), []string{layer.Path}, args[0], "download")
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, container.Id)
	return nil
}

func (srv *Server) CmdPut(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	if len(args) < 1 {
		return errors.New("Not enough arguments")
	}
	fmt.Printf("Adding layer\n")
	layer, err := srv.layers.AddLayer(stdin, stdout)
	if err != nil {
		return err
	}
	id := layer.Id()
	if !srv.docker.Exists(id) {
		log.Println("Creating new container: " + id)
		log.Printf("%v\n", srv.docker.List())
		_, err := srv.addContainer(id, []string{layer.Path}, args[0], "upload")
		if err != nil {
			return err
		}
	}
	fmt.Fprintln(stdout, id)
	return nil
}


func (srv *Server) CmdCommit(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
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
	if _, exists := srv.findContainer(srcName); exists {
		//dst := srv.addContainer(dstName, "snapshot:" + src.Id, src.Size)
		//fmt.Fprintln(stdout, dst.Id)
		return nil
	}
	return errors.New("No such container: " + srcName)
}

func (srv *Server) CmdTar(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := rcli.Subcmd(stdout,
		"tar", "CONTAINER",
		"Stream the contents of a container as a tar archive")
	if err := flags.Parse(args); err != nil {
		return nil
	}
	name := flags.Arg(0)
	if _, exists := srv.findContainer(name); exists {
		// Stream the entire contents of the container (basically a volatile snapshot)
		return fake.WriteFakeTar(stdout)
	}
	return errors.New("No such container: " + name)
}

func (srv *Server) CmdDiff(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
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
	if container, exists := srv.findContainer(flags.Arg(0)); !exists {
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
@@ -158,6 +158,7 @@ func (srv *Server) CmdDiff(stdin io.ReadCloser, stdout io.Writer, args ...str
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

type ByDate []*docker.Container

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

func (c *ByDate) Add(container *docker.Container) {
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


func (srv *Server) addContainer(id string, layers []string, name string, source string) (*fake.Container, error) {
	c, err := srv.docker.Create(id, "", nil, layers, &docker.Config{Hostname: id, Ram: 512 * 1024 * 1024})
	if err != nil {
		return nil, err
	}
	if err := c.SetUserData("name", name); err != nil {
		srv.docker.Destroy(c)
		return nil, err
	}
	if _, exists := srv.containersByName[name]; !exists {
		srv.containersByName[name] = new(ByDate)
	}
	srv.containersByName[name].Add(c)
	return fake.NewContainer(c), nil
}


func (srv *Server) rm(id string) (*docker.Container, error) {
	container := srv.docker.Get(id)
	if container == nil {
		return nil, errors.New("No such continer: " + id)
	}
	// Remove from name lookup
	srv.containersByName[container.GetUserData("name")].Del(container.Id)
	if err := srv.docker.Destroy(container); err != nil {
		return container, err
	}
	return container, nil
}


func (srv *Server) CmdLogs(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := rcli.Subcmd(stdout, "logs", "[OPTIONS] CONTAINER", "Fetch the logs of a container")
	if err := flags.Parse(args); err != nil {
		return nil
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return nil
	}
	name := flags.Arg(0)
	if container, exists := srv.findContainer(name); exists {
		if _, err := io.Copy(stdout, container.StdoutLog()); err != nil {
			return err
		}
		return nil
	}
	return errors.New("No such container: " + flags.Arg(0))
}

func (srv *Server) CmdRun(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
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
	if container, exists := srv.findContainer(name); exists {
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

func main() {
	future.Seed()
	flag.Parse()
	d, err := New()
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		if err := rcli.ListenAndServeHTTP(":8080", d); err != nil {
			log.Fatal(err)
		}
	}()
	if err := rcli.ListenAndServeTCP(":4242", d); err != nil {
		log.Fatal(err)
	}
}

func New() (*Server, error) {
	store, err := future.NewStore("/var/lib/docker/layers")
	if err != nil {
		return nil, err
	}
	if err := store.Init(); err != nil {
		return nil, err
	}
	d, err := docker.New()
	if err != nil {
		return nil, err
	}
	srv := &Server{
		containersByName: make(map[string]*ByDate),
		layers: store,
		docker: d,
	}
	// Update name index
	log.Printf("Building name index from %s...\n")
	for _, container := range srv.docker.List() {
		log.Printf("Indexing %s to %s\n", container.Id, container.GetUserData("name"))
		name := container.GetUserData("name")
		if _, exists := srv.containersByName[name]; !exists {
			srv.containersByName[name] = new(ByDate)
		}
		srv.containersByName[name].Add(container)
	}
	log.Printf("Done building index\n")
	return srv, nil
}

func (srv *Server) CmdMirror(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	_, err := io.Copy(stdout, stdin)
	return err
}

func (srv *Server) CmdDebug(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
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

func (srv *Server) CmdWeb(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
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


type Server struct {
	containersByName	map[string]*ByDate
	layers			*future.Store
	docker			*docker.Docker
}

