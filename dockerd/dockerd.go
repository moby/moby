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

func (docker *Docker) CmdUsage(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	fmt.Fprintf(stdout, "Usage: docker COMMAND [arg...]\n\nCommands:\n")
	for _, cmd := range [][]interface{}{
		{"run", "Run a command in a container"},
		{"list", "Display a list of containers"},
		{"layers", "Display a list of layers"},
		{"download", "Download a layer from a remote location"},
		{"upload", "Upload a layer"},
		{"wait", "Wait for the state of a container to change"},
		{"stop", "Stop a running container"},
		{"logs", "Fetch the logs of a container"},
		{"export", "Extract changes to a container's filesystem into a new layer"},
		{"attach", "Attach to the standard inputs and outputs of a running container"},
		{"info", "Display system-wide information"},
	} {
		fmt.Fprintf(stdout, "    %-10.10s%s\n", cmd...)
	}
	return nil
}

func (docker *Docker) CmdLayers(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	w := tabwriter.NewWriter(stdout, 20, 1, 3, ' ', 0)
	fmt.Fprintf(w, "ID\tSOURCE\tADDED\n")
	for _, layer := range docker.layers {
		fmt.Fprintf(w, "%s\t%s\t%s ago\n", layer.Id, layer.Name, time.Now().Sub(layer.Added))
	}
	w.Flush()
	return nil
}

func (docker *Docker) CmdUpload(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	layer := Layer{Id: randomId(), Name: args[0], Added: time.Now()}
	docker.layers = append(docker.layers, layer)
	fmt.Fprintf(stdout, "New layer: %s (%s)\n", layer.Id, layer.Name)
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
		Layers:	docker.layers[:1],
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
	w := tabwriter.NewWriter(stdout, 20, 1, 3, ' ', 0)
	fmt.Fprintf(w, "ID\tCMD\tSTATUS\tLAYERS\n")
	for _, container := range docker.containers {
		var layers []string
		for _, layer := range container.Layers {
			layers = append(layers, layer.Name)
		}
		fmt.Fprintf(w, "%s\t%s %s\t?\t%s\n",
			container.Id,
			container.Cmd,
			strings.Join(container.Args, " "),
			strings.Join(layers, ", "))
	}
	w.Flush()
	return nil
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	flag.Parse()
	if err := http.ListenAndServe(":4242", new(Docker)); err != nil {
		log.Fatal(err)
	}
}

func (docker *Docker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cmd, args := URLToCall(r.URL)
	method := docker.getMethod(cmd)
	if method == nil {
		docker.CmdUsage(r.Body, w, cmd)
	} else {
		err := method(r.Body, w, args...)
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
}

type Container struct {
	Id	string
	Cmd	string
	Args	[]string
	Layers	[]Layer
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
