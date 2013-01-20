package main

import (
	"errors"
	"log"
	"io"
//	"io/ioutil"
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
)

func (docker *Docker) CmdHelp(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	log.Printf("Help %s\n", args)
	if len(args) == 0 {
		fmt.Fprintf(stdout, "Usage: docker COMMAND [arg...]\n\nA self-sufficient runtime for linux containers.\n\nCommands:\n")
		for _, cmd := range [][]interface{}{
			{"run", "Run a command in a container"},
			{"clone", "Duplicate a container"},
			{"list", "Display a list of containers"},
			{"layers", "Display a list of layers"},
			{"get", "Download a layer from a remote location"},
			{"wait", "Wait for the state of a container to change"},
			{"stop", "Stop a running container"},
			{"logs", "Fetch the logs of a container"},
			{"export", "Extract changes to a container's filesystem into a new layer"},
			{"attach", "Attach to the standard inputs and outputs of a running container"},
			{"info", "Display system-wide information"},
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
	w := tabwriter.NewWriter(stdout, 20, 1, 3, ' ', 0)
	fmt.Fprintf(w, "ID\tNAME\tSIZE\tADDED\n")
	for _, layer := range docker.layers {
		fmt.Fprintf(w, "%s\t%s\t%.1fM\t%s ago\n", layer.Id, layer.Name, float32(layer.Size) / 1024 / 1024, humanDuration(time.Now().Sub(layer.Added)))
	}
	w.Flush()
	return nil
}

func (docker *Docker) CmdDownload(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	if len(args) < 1 {
		return errors.New("Not enough arguments")
	}
	fmt.Fprintf(stdout, "Downloading from %s...\n", args[0])
	time.Sleep(2 * time.Second)
	return docker.CmdUpload(stdin, stdout, args...)
	return nil
}

func (docker *Docker) CmdUpload(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	layer := Layer{Id: randomId(), Name: args[0], Added: time.Now(), Size: uint(rand.Int31n(142 * 1024 * 1024))}
	docker.layers = append(docker.layers, layer)
	time.Sleep(1 * time.Second)
	fmt.Fprintf(stdout, "New layer: %s %s %.1fM\n", layer.Id, layer.Name, float32(layer.Size) / 1024 / 1024)
	return nil
}

func (docker *Docker) CmdRun(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	if len(docker.layers) == 0 {
		return errors.New("No layers")
	}
	container := Container{
		Id:	randomId(),
		Cmd:	args[0],
		Args:	args[1:],
		Created: time.Now(),
		Layers:	docker.layers[:1],
		FilesChanged: uint(rand.Int31n(42)),
		BytesChanged: uint(rand.Int31n(24 * 1024 * 1024)),
	}
	docker.containers = append(docker.containers, container)
	cmd := exec.Command(args[0], args[1:]...)
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
	fmt.Fprintf(stdout, tpl, "ID", longestCol, longestCol, "CMD", "STATUS", "CREATED", "CHANGES", "LAYERS")
	for _, container := range docker.containers {
		var layers []string
		for _, layer := range container.Layers {
			layers = append(layers, layer.Name)
		}
		fmt.Fprintf(stdout, tpl,
			/* ID */	container.Id,
			/* CMD */	longestCol, longestCol, container.CmdString(),
			/* STATUS */	"?",
			/* CREATED */	humanDuration(time.Now().Sub(container.Created)) + " ago",
			/* CHANGES */	fmt.Sprintf("%.1fM", float32(container.BytesChanged) / 1024 / 1024),
			/* LAYERS */	strings.Join(layers, ", "))
	}
	return nil
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	flag.Parse()
	if err := http.ListenAndServe(":4242", new(Docker)); err != nil {
		log.Fatal(err)
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
	cmd, args := URLToCall(r.URL)
	log.Printf("%s\n", strings.Join(append(append([]string{"docker"}, cmd), args...), " "))
	if cmd == "" {
		docker.CmdUsage(r.Body, w, "")
		return
	}
	method := docker.getMethod(cmd)
	if method == nil {
		docker.CmdUsage(r.Body, w, cmd)
	} else {
		err := method(r.Body, &AutoFlush{w}, args...)
		if err != nil {
			fmt.Fprintf(w, "Error: %s\n", err)
		}
	}
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
	layers		[]Layer
	containers	[]Container
}

type Layer struct {
	Id	string
	Name	string
	Added	time.Time
	Size	uint
}

type Container struct {
	Id	string
	Cmd	string
	Args	[]string
	Layers	[]Layer
	Created	time.Time
	FilesChanged uint
	BytesChanged uint
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
