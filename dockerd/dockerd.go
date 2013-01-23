package main

import (
	"errors"
	"log"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os/exec"
	"flag"
	"reflect"
	"fmt"
	"github.com/kr/pty"
	"path"
	"strings"
	"time"
	"math/rand"
	"crypto/sha256"
	"bytes"
	"text/tabwriter"
	"sort"
	"os"
	"archive/tar"
)

func (docker *Docker) CmdHelp(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	if len(args) == 0 {
		fmt.Fprintf(stdout, "Usage: docker COMMAND [arg...]\n\nA self-sufficient runtime for linux containers.\n\nCommands:\n")
		for _, cmd := range [][]interface{}{
			{"run", "Run a command in a container"},
			{"list", "Display a list of containers"},
			{"get", "Download a tarball and create a container from it"},
			{"put", "Upload a tarball and create a container from it"},
			{"rm", "Remove containers"},
			{"wait", "Wait for the state of a container to change"},
			{"stop", "Stop a running container"},
			{"logs", "Fetch the logs of a container"},
			{"diff", "Inspect changes on a container's filesystem"},
			{"fork", "Duplicate a container"},
			{"attach", "Attach to the standard inputs and outputs of a running container"},
			{"info", "Display system-wide information"},
			{"web", "Generate a web UI"},
		} {
			fmt.Fprintf(stdout, "    %-10.10s%s\n", cmd...)
		}
	} else {
		if method := docker.getMethod(args[0]); method == nil {
			return errors.New("No such command: " + args[0])
		} else {
			method(stdin, stdout, "--help")
		}
	}
	return nil
}

func (docker *Docker) CmdList(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := Subcmd(stdout, "list", "[OPTIONS] [NAME]", "List containers")
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
					/* CREATED */	humanDuration(time.Now().Sub(container.Created)) + " ago",
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
	flags := Subcmd(stdout, "rm", "[OPTIONS] CONTAINER", "Remove a container")
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

func (docker *Docker) CmdGet(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	if len(args) < 1 {
		return errors.New("Not enough arguments")
	}
	time.Sleep(2 * time.Second)
	layer := docker.addContainer(args[0], "download", 0)
	fmt.Fprintln(stdout, layer.Id)
	return nil
}

func (docker *Docker) CmdPut(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	if len(args) < 1 {
		return errors.New("Not enough arguments")
	}
	time.Sleep(1 * time.Second)
	layer := docker.addContainer(args[0], "upload", 0)
	fmt.Fprintln(stdout, layer.Id)
	return nil
}

func (docker *Docker) CmdFork(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := Subcmd(stdout,
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
	flags := Subcmd(stdout,
		"tar", "CONTAINER",
		"Stream the contents of a container as a tar archive")
	if err := flags.Parse(args); err != nil {
		return nil
	}
	name := flags.Arg(0)
	if _, exists := docker.findContainer(name); exists {
		// Stream the entire contents of the container (basically a volatile snapshot)
		return WriteFakeTar(stdout)
	}
	return errors.New("No such container: " + name)
}

func (docker *Docker) CmdDiff(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := Subcmd(stdout,
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
        flags := Subcmd(stdout,
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
	// Generate a fake random size
	if size == 0 {
		size = uint(rand.Int31n(142 * 1024 * 1024))
	}
	c := &Container{
		Id:		randomId(),
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
	flags := Subcmd(stdout, "logs", "[OPTIONS] CONTAINER", "Fetch the logs of a container")
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
	flags := Subcmd(stdout, "run", "[OPTIONS] CONTAINER COMMAND [ARG...]", "Run a command in a container")
	fl_attach := flags.Bool("a", false, "Attach stdin and stdout")
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
			return container.Run(cmd[0], cmd[1:], stdin, stdout)
		} else {
			go container.Run(cmd[0], cmd[1:], ioutil.NopCloser(new(bytes.Buffer)), ioutil.Discard)
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
	rand.Seed(time.Now().UTC().UnixNano())
	flag.Parse()
	if err := http.ListenAndServe(":4242", New()); err != nil {
		log.Fatal(err)
	}
}

func New() *Docker {
	return &Docker{
		containersByName: make(map[string]*ByDate),
		containers: make(map[string]*Container),
	}
}

type AutoFlush struct {
	http.ResponseWriter
}

func (w *AutoFlush) Write(data []byte) (int, error) {
	ret, err := w.ResponseWriter.Write(data)
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
	return ret, err
}

func (docker *Docker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	stdout := &AutoFlush{w}
	stdin := r.Body
	flags := flag.NewFlagSet("docker", flag.ContinueOnError)
	flags.SetOutput(stdout)
	flags.Usage = func() { docker.CmdHelp(stdin, stdout) }
	cmd, args := URLToCall(r.URL)
	if err := flags.Parse(append([]string{cmd}, args...)); err != nil {
		return
	}
	log.Printf("%s\n", strings.Join(append(append([]string{"docker"}, cmd), args...), " "))
	if cmd == "" {
		cmd = "help"
	} else if cmd == "web" {
		w.Header().Set("content-type", "text/html")
	}
	method := docker.getMethod(cmd)
	if method == nil {
		fmt.Fprintf(stdout, "Error: no such command: %s\n", cmd)
	} else {
		err := method(stdin, stdout, args...)
		if err != nil {
			fmt.Fprintf(stdout, "Error: %s\n", err)
		}
	}
}

func (docker *Docker) CmdWeb(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := Subcmd(stdout, "web", "[OPTIONS]", "A web UI for docker")
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

func (docker *Docker) getMethod(name string) Cmd {
	methodName := "Cmd"+strings.ToUpper(name[:1])+strings.ToLower(name[1:])
	method, exists := reflect.TypeOf(docker).MethodByName(methodName)
	if !exists {
		return nil
	}
	return func(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
		ret := method.Func.CallSlice([]reflect.Value{
			reflect.ValueOf(docker),
			reflect.ValueOf(stdin),
			reflect.ValueOf(stdout),
			reflect.ValueOf(args),
		})[0].Interface()
		if ret == nil {
			return nil
		}
		return ret.(error)
	}
}

func Go(f func() error) chan error {
	ch := make(chan error)
	go func() {
		ch <- f()
	}()
	return ch
}

type Docker struct {
	containers		map[string]*Container
	containersByName	map[string]*ByDate
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

func (c *Container) Run(command string, args []string, stdin io.ReadCloser, stdout io.Writer) error {
	// Not thread-safe
	if c.Running {
		return errors.New("Already running")
	}
	c.Cmd = command
	c.Args = args
	// Reset logs
	c.stdoutLog.Reset()
	c.stdinLog.Reset()
	c.Running = true
	defer func() { c.Running = false }()
	cmd := exec.Command(c.Cmd, c.Args...)
	cmd_stdin, cmd_stdout, err := startCommand(cmd, false)
	// ADD FAKE RANDOM CHANGES
	c.FilesChanged = uint(rand.Int31n(42))
	c.BytesChanged = uint(rand.Int31n(24 * 1024 * 1024))
	if err != nil {
		return err
	}
	copy_out := Go(func() error {
		_, err := io.Copy(io.MultiWriter(stdout, c.stdoutLog), cmd_stdout)
		return err
	})
	copy_in := Go(func() error {
		//_, err := io.Copy(io.MultiWriter(cmd_stdin, c.stdinLog), stdin)
		cmd_stdin.Close()
		stdin.Close()
		//return err
		return nil
	})
	if err := cmd.Wait(); err != nil {
		return err
	}
	if err := <-copy_in; err != nil {
		return err
	}
	if err := <-copy_out; err != nil {
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

type Cmd func(io.ReadCloser, io.Writer, ...string) error
type CmdMethod func(*Docker, io.ReadCloser, io.Writer, ...string) error

// Use this key to encode an RPC call into an URL,
// eg. domain.tld/path/to/method?q=get_user&q=gordon
const ARG_URL_KEY = "q"

func URLToCall(u *url.URL) (method string, args []string) {
	return path.Base(u.Path), u.Query()[ARG_URL_KEY]
}


func randomBytes() io.Reader {
	return bytes.NewBuffer([]byte(fmt.Sprintf("%x", rand.Int())))
}

func ComputeId(content io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, content); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)[:8]), nil
}

func randomId() string {
	id, _ := ComputeId(randomBytes()) // can't fail
	return id
}


func humanDuration(d time.Duration) string {
	if seconds := int(d.Seconds()); seconds < 1 {
		return "Less than a second"
	} else if seconds < 60 {
		return fmt.Sprintf("%d seconds", seconds)
	} else if minutes := int(d.Minutes()); minutes == 1 {
		return "About a minute"
	} else if minutes < 60 {
		return fmt.Sprintf("%d minutes", minutes)
	} else if hours := int(d.Hours()); hours  == 1{
		return "About an hour"
	} else if hours < 48 {
		return fmt.Sprintf("%d hours", hours)
	} else if hours < 24 * 7 * 2 {
		return fmt.Sprintf("%d days", hours / 24)
	} else if hours < 24 * 30 * 3 {
		return fmt.Sprintf("%d weeks", hours / 24 / 7)
	} else if hours < 24 * 365 * 2 {
		return fmt.Sprintf("%d months", hours / 24 / 30)
	}
	return fmt.Sprintf("%d years", d.Hours() / 24 / 365)
}

func Subcmd(output io.Writer, name, signature, description string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(output)
	flags.Usage = func() {
		fmt.Fprintf(output, "\nUsage: docker %s %s\n\n%s\n\n", name, signature, description)
		flags.PrintDefaults()
	}
	return flags
}


func WriteFakeTar(dst io.Writer) error {
	if data, err := FakeTar(); err != nil {
		return err
	} else if _, err := io.Copy(dst, data); err != nil {
		return err
	}
	return nil
}

func FakeTar() (io.Reader, error) {
	content := []byte("Hello world!\n")
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	for _, name := range []string {"/etc/postgres/postgres.conf", "/etc/passwd", "/var/log/postgres", "/var/log/postgres/postgres.conf"} {
		hdr := new(tar.Header)
		hdr.Size = int64(len(content))
		hdr.Name = name
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		tw.Write([]byte(content))
	}
	tw.Close()
	return buf, nil
}
