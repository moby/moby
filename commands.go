package docker

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/dotcloud/docker/auth"
	"github.com/dotcloud/docker/term"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"
)

const VERSION = "0.2.2"

var (
	GIT_COMMIT string
)

func checkRemoteVersion() error {
	body, _, err := call("GET", "/version", nil)
	if err != nil {
		return fmt.Errorf("Can't connect to docker daemon. Is 'docker -d' running on this host?")
	}

	var out ApiVersion
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	if out.Version != VERSION {
		fmt.Fprintf(os.Stderr, "Warning: client and server don't have the same version (client: %s, server: %)", VERSION, out.Version)
	}
	return nil
}

func ParseCommands(args ...string) error {

	if err := checkRemoteVersion(); err != nil {
		return err
	}

	cmds := map[string]func(args ...string) error{
		"attach":  CmdAttach,
		"commit":  CmdCommit,
		"diff":    CmdDiff,
		"export":  CmdExport,
		"images":  CmdImages,
		"info":    CmdInfo,
		"inspect": CmdInspect,
		"import":  CmdImport,
		"history": CmdHistory,
		"kill":    CmdKill,
		"login":   CmdLogin,
		"logs":    CmdLogs,
		"port":    CmdPort,
		"ps":      CmdPs,
		"pull":    CmdPull,
		"push":    CmdPush,
		"restart": CmdRestart,
		"rm":      CmdRm,
		"rmi":     CmdRmi,
		"run":     CmdRun,
		"tag":     CmdTag,
		"start":   CmdStart,
		"stop":    CmdStop,
		"version": CmdVersion,
		"wait":    CmdWait,
	}

	if len(args) > 0 {
		cmd, exists := cmds[args[0]]
		if !exists {
			fmt.Println("Error: Command not found:", args[0])
			return cmdHelp(args...)
		}
		return cmd(args[1:]...)
	}
	return cmdHelp(args...)
}

func cmdHelp(args ...string) error {
	help := "Usage: docker COMMAND [arg...]\n\nA self-sufficient runtime for linux containers.\n\nCommands:\n"
	for _, cmd := range [][]string{
		{"attach", "Attach to a running container"},
		{"commit", "Create a new image from a container's changes"},
		{"diff", "Inspect changes on a container's filesystem"},
		{"export", "Stream the contents of a container as a tar archive"},
		{"history", "Show the history of an image"},
		{"images", "List images"},
		{"import", "Create a new filesystem image from the contents of a tarball"},
		{"info", "Display system-wide information"},
		{"inspect", "Return low-level information on a container/image"},
		{"kill", "Kill a running container"},
		{"login", "Register or Login to the docker registry server"},
		{"logs", "Fetch the logs of a container"},
		{"port", "Lookup the public-facing port which is NAT-ed to PRIVATE_PORT"},
		{"ps", "List containers"},
		{"pull", "Pull an image or a repository from the docker registry server"},
		{"push", "Push an image or a repository to the docker registry server"},
		{"restart", "Restart a running container"},
		{"rm", "Remove a container"},
		{"rmi", "Remove an image"},
		{"run", "Run a command in a new container"},
		{"start", "Start a stopped container"},
		{"stop", "Stop a running container"},
		{"tag", "Tag an image into a repository"},
		{"version", "Show the docker version information"},
		{"wait", "Block until a container stops, then print its exit code"},
	} {
		help += fmt.Sprintf("    %-10.10s%s\n", cmd[0], cmd[1])
	}
	fmt.Println(help)
	return nil
}

// 'docker login': login / register a user to registry service.
func CmdLogin(args ...string) error {
	var readStringOnRawTerminal = func(stdin io.Reader, stdout io.Writer, echo bool) string {
		char := make([]byte, 1)
		buffer := make([]byte, 64)
		var i = 0
		for i < len(buffer) {
			n, err := stdin.Read(char)
			if n > 0 {
				if char[0] == '\r' || char[0] == '\n' {
					stdout.Write([]byte{'\r', '\n'})
					break
				} else if char[0] == 127 || char[0] == '\b' {
					if i > 0 {
						if echo {
							stdout.Write([]byte{'\b', ' ', '\b'})
						}
						i--
					}
				} else if !unicode.IsSpace(rune(char[0])) &&
					!unicode.IsControl(rune(char[0])) {
					if echo {
						stdout.Write(char)
					}
					buffer[i] = char[0]
					i++
				}
			}
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(stdout, "Read error: %v\r\n", err)
				}
				break
			}
		}
		return string(buffer[:i])
	}
	var readAndEchoString = func(stdin io.Reader, stdout io.Writer) string {
		return readStringOnRawTerminal(stdin, stdout, true)
	}
	var readString = func(stdin io.Reader, stdout io.Writer) string {
		return readStringOnRawTerminal(stdin, stdout, false)
	}

	oldState, err := SetRawTerminal()
	if err != nil {
		return err
	} else {
		defer RestoreTerminal(oldState)
	}

	cmd := Subcmd("login", "", "Register or Login to the docker registry server")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	body, _, err := call("GET", "/auth", nil)
	if err != nil {
		return err
	}

	var out auth.AuthConfig
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}

	var username string
	var password string
	var email string

	fmt.Print("Username (", out.Username, "): ")
	username = readAndEchoString(os.Stdin, os.Stdout)
	if username == "" {
		username = out.Username
	}
	if username != out.Username {
		fmt.Print("Password: ")
		password = readString(os.Stdin, os.Stdout)

		if password == "" {
			return fmt.Errorf("Error : Password Required")
		}

		fmt.Print("Email (", out.Email, "): ")
		email = readAndEchoString(os.Stdin, os.Stdout)
		if email == "" {
			email = out.Email
		}
	} else {
		email = out.Email
	}

	out.Username = username
	out.Password = password
	out.Email = email

	body, _, err = call("POST", "/auth", out)
	if err != nil {
		return err
	}

	var out2 ApiAuth
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	if out2.Status != "" {
		fmt.Print(out2.Status)
	}
	return nil
}

// 'docker wait': block until a container stops
func CmdWait(args ...string) error {
	cmd := Subcmd("wait", "CONTAINER [CONTAINER...]", "Block until a container stops, then print its exit code.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	for _, name := range cmd.Args() {
		body, _, err := call("POST", "/containers/"+name+"/wait", nil)
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			var out ApiWait
			err = json.Unmarshal(body, &out)
			if err != nil {
				return err
			}
			fmt.Println(out.StatusCode)
		}
	}
	return nil
}

// 'docker version': show version information
func CmdVersion(args ...string) error {
	cmd := Subcmd("version", "", "Show the docker version information.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() > 0 {
		cmd.Usage()
		return nil
	}

	body, _, err := call("GET", "/version", nil)
	if err != nil {
		return err
	}

	var out ApiVersion
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	fmt.Println("Version:", out.Version)
	fmt.Println("Git Commit:", out.GitCommit)
	if !out.MemoryLimit {
		fmt.Println("WARNING: No memory limit support")
	}
	if !out.SwapLimit {
		fmt.Println("WARNING: No swap limit support")
	}

	return nil
}

// 'docker info': display system-wide information.
func CmdInfo(args ...string) error {
	cmd := Subcmd("info", "", "Display system-wide information")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() > 0 {
		cmd.Usage()
		return nil
	}

	body, _, err := call("GET", "/info", nil)
	if err != nil {
		return err
	}

	var out ApiInfo
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	fmt.Printf("containers: %d\nversion: %s\nimages: %d\n", out.Containers, out.Version, out.Images)
	if out.Debug {
		fmt.Println("debug mode enabled")
		fmt.Printf("fds: %d\ngoroutines: %d\n", out.NFd, out.NGoroutines)
	}
	return nil
}

func CmdStop(args ...string) error {
	cmd := Subcmd("stop", "[OPTIONS] CONTAINER [CONTAINER...]", "Stop a running container")
	nSeconds := cmd.Int("t", 10, "wait t seconds before killing the container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	v := url.Values{}
	v.Set("t", strconv.Itoa(*nSeconds))

	for _, name := range cmd.Args() {
		_, _, err := call("POST", "/containers/"+name+"/stop?"+v.Encode(), nil)
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

func CmdRestart(args ...string) error {
	cmd := Subcmd("restart", "[OPTIONS] CONTAINER [CONTAINER...]", "Restart a running container")
	nSeconds := cmd.Int("t", 10, "wait t seconds before killing the container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	v := url.Values{}
	v.Set("t", strconv.Itoa(*nSeconds))

	for _, name := range cmd.Args() {
		_, _, err := call("POST", "/containers/"+name+"/restart?"+v.Encode(), nil)
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

func CmdStart(args ...string) error {
	cmd := Subcmd("start", "CONTAINER [CONTAINER...]", "Restart a stopped container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range args {
		_, _, err := call("POST", "/containers/"+name+"/start", nil)
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

func CmdInspect(args ...string) error {
	cmd := Subcmd("inspect", "CONTAINER|IMAGE", "Return low-level information on a container/image")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}
	obj, _, err := call("GET", "/containers/"+cmd.Arg(0), nil)
	if err != nil {
		obj, _, err = call("GET", "/images/"+cmd.Arg(0), nil)
		if err != nil {
			return err
		}
	}
	fmt.Printf("%s\n", obj)
	return nil
}

func CmdPort(args ...string) error {
	cmd := Subcmd("port", "CONTAINER PRIVATE_PORT", "Lookup the public-facing port which is NAT-ed to PRIVATE_PORT")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 2 {
		cmd.Usage()
		return nil
	}
	v := url.Values{}
	v.Set("port", cmd.Arg(1))
	body, _, err := call("GET", "/containers/"+cmd.Arg(0)+"/port?"+v.Encode(), nil)
	if err != nil {
		return err
	}

	var out ApiPort
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	fmt.Println(out.Port)
	return nil
}

// 'docker rmi IMAGE' removes all images with the name IMAGE
func CmdRmi(args ...string) error {
	cmd := Subcmd("rmi", "IMAGE [IMAGE...]", "Remove an image")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range cmd.Args() {
		_, _, err := call("DELETE", "/images/"+name, nil)
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

func CmdHistory(args ...string) error {
	cmd := Subcmd("history", "IMAGE", "Show the history of an image")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	body, _, err := call("GET", "/images/"+cmd.Arg(0)+"/history", nil)
	if err != nil {
		return err
	}

	var outs []ApiHistory
	err = json.Unmarshal(body, &outs)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tCREATED\tCREATED BY")

	for _, out := range outs {
		fmt.Fprintf(w, "%s\t%s ago\t%s\n", out.Id, HumanDuration(time.Now().Sub(time.Unix(out.Created, 0))), out.CreatedBy)
	}
	w.Flush()
	return nil
}

func CmdRm(args ...string) error {
	cmd := Subcmd("rm", "[OPTIONS] CONTAINER [CONTAINER...]", "Remove a container")
	v := cmd.Bool("v", false, "Remove the volumes associated to the container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	val := url.Values{}
	if *v {
		val.Set("v", "1")
	}
	for _, name := range cmd.Args() {
		_, _, err := call("DELETE", "/containers/"+name+"?"+val.Encode(), nil)
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

// 'docker kill NAME' kills a running container
func CmdKill(args ...string) error {
	cmd := Subcmd("kill", "CONTAINER [CONTAINER...]", "Kill a running container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range args {
		_, _, err := call("POST", "/containers/"+name+"/kill", nil)
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

func CmdImport(args ...string) error {
	cmd := Subcmd("import", "URL|- [REPOSITORY [TAG]]", "Create a new filesystem image from the contents of a tarball")

	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	src, repository, tag := cmd.Arg(0), cmd.Arg(1), cmd.Arg(2)
	v := url.Values{}
	v.Set("repo", repository)
	v.Set("tag", tag)
	v.Set("fromSrc", src)

	err := hijack("POST", "/images?"+v.Encode(), false)
	if err != nil {
		return err
	}
	return nil
}

func CmdPush(args ...string) error {
	cmd := Subcmd("push", "NAME", "Push an image or a repository to the registry")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	name := cmd.Arg(0)

	if name == "" {
		cmd.Usage()
		return nil
	}

	body, _, err := call("GET", "/auth", nil)
	if err != nil {
		return err
	}

	var out auth.AuthConfig
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}

	// If the login failed, abort
	if out.Username == "" {
		if err := CmdLogin(args...); err != nil {
			return err
		}

		body, _, err = call("GET", "/auth", nil)
		if err != nil {
			return err
		}
		err = json.Unmarshal(body, &out)
		if err != nil {
			return err
		}

		if out.Username == "" {
			return fmt.Errorf("Please login prior to push. ('docker login')")
		}
	}

	if len(strings.SplitN(name, "/", 2)) == 1 {
		return fmt.Errorf(
			"Impossible to push a \"root\" repository. Please rename your repository in <user>/<repo> (ex: %s/%s)",
			out.Username, name)
	}

	if err := hijack("POST", "/images"+name+"/push", false); err != nil {
		return err
	}
	return nil
}

func CmdPull(args ...string) error {
	cmd := Subcmd("pull", "NAME", "Pull an image or a repository from the registry")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}
	v := url.Values{}
	v.Set("fromImage", cmd.Arg(0))

	if err := hijack("POST", "/images?"+v.Encode(), false); err != nil {
		return err
	}

	return nil
}

func CmdImages(args ...string) error {
	cmd := Subcmd("images", "[OPTIONS] [NAME]", "List images")
	quiet := cmd.Bool("q", false, "only show numeric IDs")
	all := cmd.Bool("a", false, "show all images")

	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() > 1 {
		cmd.Usage()
		return nil
	}
	v := url.Values{}
	if cmd.NArg() == 1 {
		v.Set("filter", cmd.Arg(0))
	}
	if *quiet {
		v.Set("quiet", "1")
	}
	if *all {
		v.Set("all", "1")
	}

	body, _, err := call("GET", "/images?"+v.Encode(), nil)
	if err != nil {
		return err
	}

	var outs []ApiImages
	err = json.Unmarshal(body, &outs)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	if !*quiet {
		fmt.Fprintln(w, "REPOSITORY\tTAG\tID\tCREATED")
	}

	for _, out := range outs {
		if !*quiet {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s ago\n", out.Repository, out.Tag, out.Id, HumanDuration(time.Now().Sub(time.Unix(out.Created, 0))))
		} else {
			fmt.Fprintln(w, out.Id)
		}
	}

	if !*quiet {
		w.Flush()
	}
	return nil
}

func CmdPs(args ...string) error {
	cmd := Subcmd("ps", "[OPTIONS]", "List containers")
	quiet := cmd.Bool("q", false, "Only display numeric IDs")
	all := cmd.Bool("a", false, "Show all containers. Only running containers are shown by default.")
	noTrunc := cmd.Bool("notrunc", false, "Don't truncate output")
	nLatest := cmd.Bool("l", false, "Show only the latest created container, include non-running ones.")
	last := cmd.Int("n", -1, "Show n last created containers, include non-running ones.")

	if err := cmd.Parse(args); err != nil {
		return nil
	}
	v := url.Values{}
	if *last == -1 && *nLatest {
		*last = 1
	}
	if *quiet {
		v.Set("quiet", "1")
	}
	if *all {
		v.Set("all", "1")
	}
	if *noTrunc {
		v.Set("notrunc", "1")
	}
	if *last != -1 {
		v.Set("n", strconv.Itoa(*last))
	}

	body, _, err := call("GET", "/containers?"+v.Encode(), nil)
	if err != nil {
		return err
	}

	var outs []ApiContainers
	err = json.Unmarshal(body, &outs)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	if !*quiet {
		fmt.Fprintln(w, "ID\tIMAGE\tCOMMAND\tCREATED\tSTATUS\tPORTS")
	}

	for _, out := range outs {
		if !*quiet {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s ago\t%s\n", out.Id, out.Image, out.Command, out.Status, HumanDuration(time.Now().Sub(time.Unix(out.Created, 0))), out.Ports)
		} else {
			fmt.Fprintln(w, out.Id)
		}
	}

	if !*quiet {
		w.Flush()
	}
	return nil
}

func CmdCommit(args ...string) error {
	cmd := Subcmd("commit", "[OPTIONS] CONTAINER [REPOSITORY [TAG]]", "Create a new image from a container's changes")
	flComment := cmd.String("m", "", "Commit message")
	flAuthor := cmd.String("author", "", "Author (eg. \"John Hannibal Smith <hannibal@a-team.com>\"")
	flConfig := cmd.String("run", "", "Config automatically applied when the image is run. "+`(ex: {"Cmd": ["cat", "/world"], "PortSpecs": ["22"]}')`)
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	name, repository, tag := cmd.Arg(0), cmd.Arg(1), cmd.Arg(2)
	if name == "" {
		cmd.Usage()
		return nil
	}

	v := url.Values{}
	v.Set("container", name)
	v.Set("repo", repository)
	v.Set("tag", tag)
	v.Set("comment", *flComment)
	v.Set("author", *flAuthor)
	var config *Config
	if *flConfig != "" {
		config = &Config{}
		if err := json.Unmarshal([]byte(*flConfig), config); err != nil {
			return err
		}
	}

	body, _, err := call("POST", "/commit?"+v.Encode(), config)
	if err != nil {
		return err
	}

	var out ApiId
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}

	fmt.Println(out.Id)
	return nil
}

func CmdExport(args ...string) error {
	cmd := Subcmd("export", "CONTAINER", "Export the contents of a filesystem as a tar archive")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	if err := hijack("GET", "/containers/"+cmd.Arg(0)+"/export", false); err != nil {
		return err
	}
	return nil
}

func CmdDiff(args ...string) error {
	cmd := Subcmd("diff", "CONTAINER", "Inspect changes on a container's filesystem")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	body, _, err := call("GET", "/containers/"+cmd.Arg(0)+"/changes", nil)
	if err != nil {
		return err
	}

	var changes []string
	err = json.Unmarshal(body, &changes)
	if err != nil {
		return err
	}
	for _, change := range changes {
		fmt.Println(change)
	}
	return nil
}

func CmdLogs(args ...string) error {
	cmd := Subcmd("logs", "CONTAINER", "Fetch the logs of a container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	v := url.Values{}
	v.Set("logs", "1")
	v.Set("stdout", "1")
	v.Set("stderr", "1")

	if err := hijack("POST", "/containers/"+cmd.Arg(0)+"/attach?"+v.Encode(), false); err != nil {
		return err
	}
	return nil
}

func CmdAttach(args ...string) error {
	cmd := Subcmd("attach", "CONTAINER", "Attach to a running container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	body, _, err := call("GET", "/containers/"+cmd.Arg(0), nil)
	if err != nil {
		return err
	}

	var container Container
	err = json.Unmarshal(body, &container)
	if err != nil {
		return err
	}

	v := url.Values{}
	v.Set("logs", "1")
	v.Set("stream", "1")
	v.Set("stdout", "1")
	v.Set("stderr", "1")
	v.Set("stdin", "1")

	if err := hijack("POST", "/containers/"+cmd.Arg(0)+"/attach?"+v.Encode(), container.Config.Tty); err != nil {
		return err
	}
	return nil
}

// ListOpts type
type ListOpts []string

func (opts *ListOpts) String() string {
	return fmt.Sprint(*opts)
}

func (opts *ListOpts) Set(value string) error {
	*opts = append(*opts, value)
	return nil
}

// AttachOpts stores arguments to 'docker run -a', eg. which streams to attach to
type AttachOpts map[string]bool

func NewAttachOpts() AttachOpts {
	return make(AttachOpts)
}

func (opts AttachOpts) String() string {
	// Cast to underlying map type to avoid infinite recursion
	return fmt.Sprintf("%v", map[string]bool(opts))
}

func (opts AttachOpts) Set(val string) error {
	if val != "stdin" && val != "stdout" && val != "stderr" {
		return fmt.Errorf("Unsupported stream name: %s", val)
	}
	opts[val] = true
	return nil
}

func (opts AttachOpts) Get(val string) bool {
	if res, exists := opts[val]; exists {
		return res
	}
	return false
}

// PathOpts stores a unique set of absolute paths
type PathOpts map[string]struct{}

func NewPathOpts() PathOpts {
	return make(PathOpts)
}

func (opts PathOpts) String() string {
	return fmt.Sprintf("%v", map[string]struct{}(opts))
}

func (opts PathOpts) Set(val string) error {
	if !filepath.IsAbs(val) {
		return fmt.Errorf("%s is not an absolute path", val)
	}
	opts[filepath.Clean(val)] = struct{}{}
	return nil
}

func CmdTag(args ...string) error {
	cmd := Subcmd("tag", "[OPTIONS] IMAGE REPOSITORY [TAG]", "Tag an image into a repository")
	force := cmd.Bool("f", false, "Force")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 2 && cmd.NArg() != 3 {
		cmd.Usage()
		return nil
	}

	v := url.Values{}
	v.Set("repo", cmd.Arg(1))
	if cmd.NArg() == 3 {
		v.Set("tag", cmd.Arg(2))
	}

	if *force {
		v.Set("force", "1")
	}

	if _, _, err := call("POST", "/images/"+cmd.Arg(0)+"/tag?"+v.Encode(), nil); err != nil {
		return err
	}
	return nil
}

func CmdRun(args ...string) error {
	config, cmd, err := ParseRun(args)
	if err != nil {
		return err
	}
	if config.Image == "" {
		cmd.Usage()
		return nil
	}
	if len(config.Cmd) == 0 {
		cmd.Usage()
		return nil
	}

	//create the container
	body, statusCode, err := call("POST", "/containers", *config)

	//if image not found try to pull it
	if statusCode == 404 {
		v := url.Values{}
		v.Set("fromImage", config.Image)
		err = hijack("POST", "/images?"+v.Encode(), false)
		if err != nil {
			return err
		}
		body, _, err = call("POST", "/containers", *config)
		if err != nil {
			return err
		}
		return nil

	}
	if err != nil {
		return err
	}

	var out ApiRun
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}

	for _, warning := range out.Warnings {
		fmt.Fprintln(os.Stderr, "WARNING: ", warning)
	}

	v := url.Values{}
	v.Set("logs", "1")
	v.Set("stream", "1")

	if config.AttachStdin {
		v.Set("stdin", "1")
	}
	if config.AttachStdout {
		v.Set("stdout", "1")
	}
	if config.AttachStderr {
		v.Set("stderr", "1")

	}

	//start the container
	_, _, err = call("POST", "/containers/"+out.Id+"/start", nil)
	if err != nil {
		return err
	}
	if config.AttachStdin || config.AttachStdout || config.AttachStderr {
		if err := hijack("POST", "/containers/"+out.Id+"/attach?"+v.Encode(), config.Tty); err != nil {
			return err
		}
		body, _, err := call("POST", "/containers/"+out.Id+"/wait", nil)
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			var out ApiWait
			err = json.Unmarshal(body, &out)
			if err != nil {
				return err
			}
			os.Exit(out.StatusCode)
		}

	} else {
		fmt.Println(out.Id)
	}
	return nil
}

func call(method, path string, data interface{}) ([]byte, int, error) {
	var params io.Reader
	if data != nil {
		buf, err := json.Marshal(data)
		if err != nil {
			return nil, -1, err
		}
		params = bytes.NewBuffer(buf)
	}

	req, err := http.NewRequest(method, "http://0.0.0.0:4243"+path, params)
	if err != nil {
		return nil, -1, err
	}
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	} else if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, -1, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, -1, err
	}
	if resp.StatusCode != 200 {
		return nil, resp.StatusCode, fmt.Errorf("error: %s", body)
	}
	return body, resp.StatusCode, nil

}

func hijack(method, path string, setRawTerminal bool) error {
	req, err := http.NewRequest(method, path, nil)
	if err != nil {
		return err
	}
	dial, err := net.Dial("tcp", "0.0.0.0:4243")
	if err != nil {
		return err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	clientconn.Do(req)
	defer clientconn.Close()

	rwc, br := clientconn.Hijack()
	defer rwc.Close()

	receiveStdout := Go(func() error {
		_, err := io.Copy(os.Stdout, br)
		return err
	})

	if setRawTerminal && term.IsTerminal(int(os.Stdin.Fd())) && os.Getenv("NORAW") == "" {
		if oldState, err := SetRawTerminal(); err != nil {
			return err
		} else {
			defer RestoreTerminal(oldState)
		}
	}

	sendStdin := Go(func() error {
		_, err := io.Copy(rwc, os.Stdin)
		rwc.Close()
		return err
	})

	if err := <-receiveStdout; err != nil {
		return err
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		if err := <-sendStdin; err != nil {
			return err
		}
	}

	return nil

}

func Subcmd(name, signature, description string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.Usage = func() {
		fmt.Printf("\nUsage: docker %s %s\n\n%s\n\n", name, signature, description)
		flags.PrintDefaults()
	}
	return flags
}
