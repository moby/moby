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
			{"clone", "Duplicate a container"},
			{"list", "Display a list of containers"},
			{"layers", "Display a list of layers"},
			{"get", "Download a layer from a remote location"},
			{"rm", "Remove layers"},
			{"wait", "Wait for the state of a container to change"},
			{"stop", "Stop a running container"},
			{"logs", "Fetch the logs of a container"},
			{"diff", "Inspect changes on a container's filesystem"},
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

func (docker *Docker) CmdLayers(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := Subcmd(stdout, "layers", "[OPTIONS] [NAME]", "Show available filesystem layers")
	quiet := flags.Bool("q", false, "Quiet mode")
	limit := flags.Int("l", 0, "Only show the N most recent versions of each layer")
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
	for name := range docker.layersByName {
		names = append(names, name)
	}
	sort.Strings(names)
	w := tabwriter.NewWriter(stdout, 20, 1, 3, ' ', 0)
	if (!*quiet) {
		fmt.Fprintf(w, "ID\tNAME\tSIZE\tADDED\tSOURCE\n")
	}
	for _, name := range names {
		if nameFilter != "" && nameFilter != name {
			continue
		}
		for idx, layer := range *docker.layersByName[name] {
			if *limit > 0 && idx >= *limit {
				break
			}
			if !*quiet {
				fmt.Fprintf(w, "%s\t%s\t%.1fM\t%s ago\t%s\n", layer.Id, layer.Name, float32(layer.Size) / 1024 / 1024, humanDuration(time.Now().Sub(layer.Added)), layer.Source)
			} else {
				stdout.Write([]byte(layer.Id + "\n"))
			}
		}
	}
	if (!*quiet) {
		w.Flush()
	}
	return nil
}

func (docker *Docker) findLayer(name string) (*Layer, bool) {
	// 1: look for layer by ID
	if layer, exists := docker.layers[name]; exists {
		return layer, true
	}
	// 2: look for a layer by name (and pick the most recent)
	if layers, exists := docker.layersByName[name]; exists {
		return (*layers)[0], true
	}
	return nil, false
}

func (docker *Docker) usingLayer(layer *Layer) []*Container {
	var containers []*Container
	for _, container := range docker.containers {
		for _, l := range container.Layers {
			if l.Id == layer.Id {
				containers = append(containers, &container)
			}
		}
	}
	return containers
}

func (docker *Docker) CmdRm(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := Subcmd(stdout, "rm", "[OPTIONS LAYER", "Remove a layer")
	if err := flags.Parse(args); err != nil {
		return nil
	}
	for _, name := range flags.Args() {
		if _, err := docker.rmLayer(name); err != nil {
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
	layer := docker.addLayer(args[0], "download", 0)
	fmt.Fprintln(stdout, layer.Id)
	return nil
}

func (docker *Docker) CmdPut(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	if len(args) < 1 {
		return errors.New("Not enough arguments")
	}
	time.Sleep(1 * time.Second)
	layer := docker.addLayer(args[0], "upload", 0)
	fmt.Fprintln(stdout, layer.Id)
	return nil
}

func (docker *Docker) CmdDiff(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := Subcmd(stdout,
		"diff", "CONTAINER [OPTIONS]",
		"Inspect changes on a container's filesystem")
	fl_bytes := flags.Bool("b", false, "Show how many bytes have been changes")
	fl_list := flags.Bool("l", false, "Show a list of changed files")
	fl_download := flags.Bool("d", false, "Download the changes as gzipped tar stream")
	fl_create := flags.String("c", "", "Create a new layer from the changes")
	if err := flags.Parse(args); err != nil {
		return nil
	}
	if flags.NArg() < 1 {
		return errors.New("Not enough arguments")
	}
	if !(*fl_bytes || *fl_list || *fl_download || *fl_create != "") {
		flags.Usage()
		return nil
	}
	if container, exists := docker.containers[flags.Arg(0)]; !exists {
		return errors.New("No such container")
	} else if *fl_bytes {
		if *fl_list || *fl_download || *fl_create != "" {
			flags.Usage()
			return nil
		}
		fmt.Fprintf(stdout, "%d\n", container.BytesChanged)
	} else if *fl_list {
		if *fl_bytes || *fl_download || *fl_create != "" {
			flags.Usage()
			return nil
		}
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
	} else if *fl_download {
		if *fl_bytes || *fl_list || *fl_create != "" {
			flags.Usage()
			return nil
		}
		if data, err := FakeTar(); err != nil {
			return err
		} else if _, err := io.Copy(stdout, data); err != nil {
			return err
		}
		return nil
	} else if *fl_create != "" {
		if *fl_bytes || *fl_list || *fl_download {
			flags.Usage()
			return nil
		}
		layer := docker.addLayer(*fl_create, "export:" + container.Id, container.BytesChanged)
		fmt.Fprintln(stdout, layer.Id)
	} else {
		flags.Usage()
		return nil
	}
	return nil
}


// ByDate wraps an array of layers so they can be sorted by date (most recent first)

type ByDate []*Layer

func (l *ByDate) Len() int {
	return len(*l)
}

func (l *ByDate) Less(i, j int) bool {
	layers := *l
	return layers[j].Added.Before(layers[i].Added)
}

func (l *ByDate) Swap(i, j int) {
	layers := *l
	tmp := layers[i]
	layers[i] = layers[j]
	layers[j] = tmp
}

func (l *ByDate) Add(layer *Layer) {
	*l = append(*l, layer)
	sort.Sort(l)
}

func (l *ByDate) Del(id string) {
	for idx, layer := range *l {
		if layer.Id == id {
			*l = append((*l)[:idx], (*l)[idx + 1:]...)
		}
	}
}


func (docker *Docker) addLayer(name string, source string, size uint) *Layer {
	if size == 0 {
		size = uint(rand.Int31n(142 * 1024 * 1024))
	}
	layer := &Layer{Id: randomId(), Name: name, Source: source, Added: time.Now(), Size: size}
	docker.layers[layer.Id] = layer
	if _, exists := docker.layersByName[layer.Name]; !exists {
		docker.layersByName[layer.Name] = new(ByDate)
	}
	docker.layersByName[layer.Name].Add(layer)
	return layer
}

func (docker *Docker) rmLayer(id string) (*Layer, error) {
	if layer, exists := docker.layers[id]; exists {
		if containers := docker.usingLayer(layer); len(containers) > 0 {
			return nil, errors.New(fmt.Sprintf("Layer is in use: %s", id))
		} else {
			// Remove from name lookup
			docker.layersByName[layer.Name].Del(layer.Id)
			// Remove from id lookup
			delete(docker.layers, layer.Id)
			return layer, nil
		}
	}
	return nil, errors.New(fmt.Sprintf("No such layer: %s", id))
}

type ArgList []string

func (l *ArgList) Set(value string) error {
	*l = append(*l, value)
	return nil
}

func (l *ArgList) String() string {
	return strings.Join(*l, ",")
}

func (docker *Docker) CmdRun(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := Subcmd(stdout, "run", "-l LAYER [-l LAYER...] COMMAND {ARG...]", "Run a command in a container")
	fl_layers := new(ArgList)
	flags.Var(fl_layers, "l", "Add a layer to the filesystem. Multiple layers are added in the order they are defined")
	fl_attach := flags.Bool("a", false, "Attach stdin and stdout")
	if err := flags.Parse(args); err != nil {
		return nil
	}
	if len(*fl_layers) < 1 {
		return errors.New("Please specify at least one layer")
	}
	if flags.NArg() < 1 {
		return errors.New("No command specified")
	}
	cmd := flags.Arg(0)
	var cmd_args []string
	if flags.NArg() > 1 {
		cmd_args = flags.Args()[1:]
	}
	container := Container{
		Id:	randomId(),
		Cmd:	cmd,
		Args:	cmd_args,
		Created: time.Now(),
		FilesChanged: uint(rand.Int31n(42)),
		BytesChanged: uint(rand.Int31n(24 * 1024 * 1024)),
	}
	for _, name := range *fl_layers {
		if layer, exists := docker.findLayer(name); exists {
			container.Layers = append(container.Layers, layer)
		} else if srcContainer, exists := docker.containers[name]; exists {
			for _, layer := range srcContainer.Layers {
				container.Layers = append(container.Layers, layer)
			}
		} else {
			return errors.New("No such layer or container: " + name)
		}
	}
	docker.containers[container.Id] = container
	if *fl_attach {
		return container.Run(stdin, stdout)
	} else {
		go container.Run(ioutil.NopCloser(new(bytes.Buffer)), ioutil.Discard)
		fmt.Fprintln(stdout, container.Id)
	}
	return nil
}

func (docker *Docker) CmdClone(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := Subcmd(stdout, "Clone", "[OPTIONS] CONTAINER_ID", "Duplicate a container")
	reset := flags.Bool("r", true, "Reset: don't keep filesystem changes from the source container")
	flags.Parse(args)
	if !*reset {
		return errors.New("Only reset mode is available for now. Please use -r")
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return nil
	}
	container, exists := docker.containers[flags.Arg(0)];
	if !exists {
		return errors.New("No such container: " + flags.Arg(0))
	}
	return docker.CmdRun(stdin, stdout, append([]string{"-l", container.Id, "--", container.Cmd}, container.Args...)...)
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

func (docker *Docker) CmdList(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	flags := Subcmd(stdout, "list", "[OPTIONS]", "Show all containers")
	numeric := flags.Bool("n", false, "Display absolute layer IDs instead of names")
	flags.Parse(args)
	var longestCol int
	for _, container := range docker.containers {
		if l := len(container.CmdString()); l > longestCol {
			longestCol = l
		}
	}
	if longestCol > 50 {
		longestCol = 50
	} else if longestCol < 5 {
		longestCol = 8
	}
	tpl := "%-16s   %-*.*s   %-6s   %-25s   %10s   %-s\n"
	fmt.Fprintf(stdout, tpl, "ID", longestCol, longestCol, "CMD", "RUNNING", "CREATED", "CHANGES", "LAYERS")
	for _, container := range docker.containers {
		var layers []string
		for _, layer := range container.Layers {
			if *numeric {
				layers = append(layers, layer.Id)
			} else {
				layers = append(layers, layer.Name)
			}
		}
		fmt.Fprintf(stdout, tpl,
			/* ID */	container.Id,
			/* CMD */	longestCol, longestCol, container.CmdString(),
			/* RUNNING */	fmt.Sprintf("%v", container.Running),
			/* CREATED */	humanDuration(time.Now().Sub(container.Created)) + " ago",
			/* CHANGES */	fmt.Sprintf("%.1fM", float32(container.BytesChanged) / 1024 / 1024),
			/* LAYERS */	strings.Join(layers, ","))
	}
	return nil
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
		layers: make(map[string]*Layer),
		layersByName: make(map[string]*ByDate),
		containers: make(map[string]Container),
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
	layers		map[string]*Layer
	layersByName	map[string]*ByDate
	containers	map[string]Container
}

type Layer struct {
	Id	string
	Name	string
	Added	time.Time
	Size	uint
	Source	string
}

type Container struct {
	Id	string
	Cmd	string
	Args	[]string
	Layers	[]*Layer
	Created	time.Time
	FilesChanged uint
	BytesChanged uint
	Running	bool
}

func (c *Container) Run(stdin io.ReadCloser, stdout io.Writer) error {
	// Not thread-safe
	if c.Running {
		return errors.New("Already running")
	}
	c.Running = true
	defer func() { c.Running = false }()
	cmd := exec.Command(c.Cmd, c.Args...)
	cmd_stdin, cmd_stdout, err := startCommand(cmd, false)
	if err != nil {
		return err
	}
	copy_out := Go(func() error {
		_, err := io.Copy(stdout, cmd_stdout)
		return err
	})
	copy_in := Go(func() error {
		//_, err := io.Copy(cmd_stdin, stdin)
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
