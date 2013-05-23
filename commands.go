package docker

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/dotcloud/docker/auth"
	"github.com/dotcloud/docker/term"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"
)

const VERSION = "0.3.2"

var (
	GIT_COMMIT string
)

func ParseCommands(args ...string) error {
	cli := NewDockerCli("0.0.0.0", 4243)

	if len(args) > 0 {
		methodName := "Cmd" + strings.ToUpper(args[0][:1]) + strings.ToLower(args[0][1:])
		method, exists := reflect.TypeOf(cli).MethodByName(methodName)
		if !exists {
			fmt.Println("Error: Command not found:", args[0])
			return cli.CmdHelp(args...)
		}
		ret := method.Func.CallSlice([]reflect.Value{
			reflect.ValueOf(cli),
			reflect.ValueOf(args[1:]),
		})[0].Interface()
		if ret == nil {
			return nil
		}
		return ret.(error)
	}
	return cli.CmdHelp(args...)
}

func (cli *DockerCli) CmdHelp(args ...string) error {
	help := "Usage: docker COMMAND [arg...]\n\nA self-sufficient runtime for linux containers.\n\nCommands:\n"
	for cmd, description := range map[string]string{
		"attach":  "Attach to a running container",
		"build":   "Build a container from Dockerfile or via stdin",
		"commit":  "Create a new image from a container's changes",
		"diff":    "Inspect changes on a container's filesystem",
		"export":  "Stream the contents of a container as a tar archive",
		"history": "Show the history of an image",
		"images":  "List images",
		"import":  "Create a new filesystem image from the contents of a tarball",
		"info":    "Display system-wide information",
		"insert":  "Insert a file in an image",
		"inspect": "Return low-level information on a container",
		"kill":    "Kill a running container",
		"login":   "Register or Login to the docker registry server",
		"logs":    "Fetch the logs of a container",
		"port":    "Lookup the public-facing port which is NAT-ed to PRIVATE_PORT",
		"ps":      "List containers",
		"pull":    "Pull an image or a repository from the docker registry server",
		"push":    "Push an image or a repository to the docker registry server",
		"restart": "Restart a running container",
		"rm":      "Remove a container",
		"rmi":     "Remove an image",
		"run":     "Run a command in a new container",
		"search":  "Search for an image in the docker index",
		"start":   "Start a stopped container",
		"stop":    "Stop a running container",
		"tag":     "Tag an image into a repository",
		"version": "Show the docker version information",
		"wait":    "Block until a container stops, then print its exit code",
	} {
		help += fmt.Sprintf("    %-10.10s%s\n", cmd, description)
	}
	fmt.Println(help)
	return nil
}

func (cli *DockerCli) CmdInsert(args ...string) error {
	cmd := Subcmd("insert", "IMAGE URL PATH", "Insert a file from URL in the IMAGE at PATH")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 3 {
		cmd.Usage()
		return nil
	}

	v := url.Values{}
	v.Set("url", cmd.Arg(1))
	v.Set("path", cmd.Arg(2))

	err := cli.stream("POST", "/images/"+cmd.Arg(0)+"/insert?"+v.Encode(), nil, os.Stdout)
	if err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) CmdBuild(args ...string) error {
	cmd := Subcmd("build", "-|Dockerfile", "Build an image from Dockerfile or via stdin")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	var (
		file io.ReadCloser
		err  error
	)

	if cmd.NArg() == 0 {
		file, err = os.Open("Dockerfile")
		if err != nil {
			return err
		}
	} else if cmd.Arg(0) == "-" {
		file = os.Stdin
	} else {
		file, err = os.Open(cmd.Arg(0))
		if err != nil {
			return err
		}
	}
	if _, err := NewBuilderClient("0.0.0.0", 4243).Build(file); err != nil {
		return err
	}
	return nil
}

// 'docker login': login / register a user to registry service.
func (cli *DockerCli) CmdLogin(args ...string) error {
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

	oldState, err := term.SetRawTerminal()
	if err != nil {
		return err
	} else {
		defer term.RestoreTerminal(oldState)
	}

	cmd := Subcmd("login", "", "Register or Login to the docker registry server")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	body, _, err := cli.call("GET", "/auth", nil)
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

	body, _, err = cli.call("POST", "/auth", out)
	if err != nil {
		return err
	}

	var out2 ApiAuth
	err = json.Unmarshal(body, &out2)
	if err != nil {
		return err
	}
	if out2.Status != "" {
		term.RestoreTerminal(oldState)
		fmt.Print(out2.Status)
	}
	return nil
}

// 'docker wait': block until a container stops
func (cli *DockerCli) CmdWait(args ...string) error {
	cmd := Subcmd("wait", "CONTAINER [CONTAINER...]", "Block until a container stops, then print its exit code.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	for _, name := range cmd.Args() {
		body, _, err := cli.call("POST", "/containers/"+name+"/wait", nil)
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
func (cli *DockerCli) CmdVersion(args ...string) error {
	cmd := Subcmd("version", "", "Show the docker version information.")
	fmt.Println(len(args))
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	fmt.Println(cmd.NArg())
	if cmd.NArg() > 0 {
		cmd.Usage()
		return nil
	}

	body, _, err := cli.call("GET", "/version", nil)
	if err != nil {
		return err
	}

	var out ApiVersion
	err = json.Unmarshal(body, &out)
	if err != nil {
		utils.Debugf("Error unmarshal: body: %s, err: %s\n", body, err)
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
func (cli *DockerCli) CmdInfo(args ...string) error {
	cmd := Subcmd("info", "", "Display system-wide information")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() > 0 {
		cmd.Usage()
		return nil
	}

	body, _, err := cli.call("GET", "/info", nil)
	if err != nil {
		return err
	}

	var out ApiInfo
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	fmt.Printf("containers: %d\nversion: %s\nimages: %d\nGo version: %s\n", out.Containers, out.Version, out.Images, out.GoVersion)
	if out.Debug {
		fmt.Println("debug mode enabled")
		fmt.Printf("fds: %d\ngoroutines: %d\n", out.NFd, out.NGoroutines)
	}
	return nil
}

func (cli *DockerCli) CmdStop(args ...string) error {
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
		_, _, err := cli.call("POST", "/containers/"+name+"/stop?"+v.Encode(), nil)
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

func (cli *DockerCli) CmdRestart(args ...string) error {
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
		_, _, err := cli.call("POST", "/containers/"+name+"/restart?"+v.Encode(), nil)
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

func (cli *DockerCli) CmdStart(args ...string) error {
	cmd := Subcmd("start", "CONTAINER [CONTAINER...]", "Restart a stopped container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range args {
		_, _, err := cli.call("POST", "/containers/"+name+"/start", nil)
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

func (cli *DockerCli) CmdInspect(args ...string) error {
	cmd := Subcmd("inspect", "CONTAINER|IMAGE", "Return low-level information on a container/image")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}
	obj, _, err := cli.call("GET", "/containers/"+cmd.Arg(0)+"/json", nil)
	if err != nil {
		obj, _, err = cli.call("GET", "/images/"+cmd.Arg(0)+"/json", nil)
		if err != nil {
			return err
		}
	}

	indented := new(bytes.Buffer)
	if err = json.Indent(indented, obj, "", "    "); err != nil {
		return err
	}
	if _, err := io.Copy(os.Stdout, indented); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) CmdPort(args ...string) error {
	cmd := Subcmd("port", "CONTAINER PRIVATE_PORT", "Lookup the public-facing port which is NAT-ed to PRIVATE_PORT")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 2 {
		cmd.Usage()
		return nil
	}

	body, _, err := cli.call("GET", "/containers/"+cmd.Arg(0)+"/json", nil)
	if err != nil {
		return err
	}
	var out Container
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}

	if frontend, exists := out.NetworkSettings.PortMapping[cmd.Arg(1)]; exists {
		fmt.Println(frontend)
	} else {
		return fmt.Errorf("error: No private port '%s' allocated on %s", cmd.Arg(1), cmd.Arg(0))
	}
	return nil
}

// 'docker rmi IMAGE' removes all images with the name IMAGE
func (cli *DockerCli) CmdRmi(args ...string) error {
	cmd := Subcmd("rmi", "IMAGE [IMAGE...]", "Remove an image")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range cmd.Args() {
		_, _, err := cli.call("DELETE", "/images/"+name, nil)
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

func (cli *DockerCli) CmdHistory(args ...string) error {
	cmd := Subcmd("history", "IMAGE", "Show the history of an image")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	body, _, err := cli.call("GET", "/images/"+cmd.Arg(0)+"/history", nil)
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
		fmt.Fprintf(w, "%s\t%s ago\t%s\n", out.Id, utils.HumanDuration(time.Now().Sub(time.Unix(out.Created, 0))), out.CreatedBy)
	}
	w.Flush()
	return nil
}

func (cli *DockerCli) CmdRm(args ...string) error {
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
		_, _, err := cli.call("DELETE", "/containers/"+name+"?"+val.Encode(), nil)
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

// 'docker kill NAME' kills a running container
func (cli *DockerCli) CmdKill(args ...string) error {
	cmd := Subcmd("kill", "CONTAINER [CONTAINER...]", "Kill a running container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range args {
		_, _, err := cli.call("POST", "/containers/"+name+"/kill", nil)
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

func (cli *DockerCli) CmdImport(args ...string) error {
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

	err := cli.stream("POST", "/images/create?"+v.Encode(), os.Stdin, os.Stdout)
	if err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) CmdPush(args ...string) error {
	cmd := Subcmd("push", "[OPTION] NAME", "Push an image or a repository to the registry")
	registry := cmd.String("registry", "", "Registry host to push the image to")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	name := cmd.Arg(0)

	if name == "" {
		cmd.Usage()
		return nil
	}

	body, _, err := cli.call("GET", "/auth", nil)
	if err != nil {
		return err
	}

	var out auth.AuthConfig
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}

	// If the login failed AND we're using the index, abort
	if *registry == "" && out.Username == "" {
		if err := cli.CmdLogin(args...); err != nil {
			return err
		}

		body, _, err = cli.call("GET", "/auth", nil)
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
		return fmt.Errorf("Impossible to push a \"root\" repository. Please rename your repository in <user>/<repo> (ex: %s/%s)", out.Username, name)
	}

	v := url.Values{}
	v.Set("registry", *registry)
	if err := cli.stream("POST", "/images/"+name+"/push?"+v.Encode(), nil, os.Stdout); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) CmdPull(args ...string) error {
	cmd := Subcmd("pull", "NAME", "Pull an image or a repository from the registry")
	tag := cmd.String("t", "", "Download tagged image in repository")
	registry := cmd.String("registry", "", "Registry to download from. Necessary if image is pulled by ID")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	remote := cmd.Arg(0)
	if strings.Contains(remote, ":") {
		remoteParts := strings.Split(remote, ":")
		tag = &remoteParts[1]
		remote = remoteParts[0]
	}

	v := url.Values{}
	v.Set("fromImage", remote)
	v.Set("tag", *tag)
	v.Set("registry", *registry)

	if err := cli.stream("POST", "/images/create?"+v.Encode(), nil, os.Stdout); err != nil {
		return err
	}

	return nil
}

func (cli *DockerCli) CmdImages(args ...string) error {
	cmd := Subcmd("images", "[OPTIONS] [NAME]", "List images")
	quiet := cmd.Bool("q", false, "only show numeric IDs")
	all := cmd.Bool("a", false, "show all images")
	noTrunc := cmd.Bool("notrunc", false, "Don't truncate output")
	flViz := cmd.Bool("viz", false, "output graph in graphviz format")

	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() > 1 {
		cmd.Usage()
		return nil
	}

	if *flViz {
		body, _, err := cli.call("GET", "/images/viz", false)
		if err != nil {
			return err
		}
		fmt.Printf("%s", body)
	} else {
		v := url.Values{}
		if cmd.NArg() == 1 {
			v.Set("filter", cmd.Arg(0))
		}
		if *all {
			v.Set("all", "1")
		}

		body, _, err := cli.call("GET", "/images/json?"+v.Encode(), nil)
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
			if out.Repository == "" {
				out.Repository = "<none>"
			}
			if out.Tag == "" {
				out.Tag = "<none>"
			}

			if !*quiet {
				fmt.Fprintf(w, "%s\t%s\t", out.Repository, out.Tag)
				if *noTrunc {
					fmt.Fprintf(w, "%s\t", out.Id)
				} else {
					fmt.Fprintf(w, "%s\t", utils.TruncateId(out.Id))
				}
				fmt.Fprintf(w, "%s ago\n", utils.HumanDuration(time.Now().Sub(time.Unix(out.Created, 0))))
			} else {
				if *noTrunc {
					fmt.Fprintln(w, out.Id)
				} else {
					fmt.Fprintln(w, utils.TruncateId(out.Id))
				}
			}
		}

		if !*quiet {
			w.Flush()
		}
	}
	return nil
}

func (cli *DockerCli) CmdPs(args ...string) error {
	cmd := Subcmd("ps", "[OPTIONS]", "List containers")
	quiet := cmd.Bool("q", false, "Only display numeric IDs")
	all := cmd.Bool("a", false, "Show all containers. Only running containers are shown by default.")
	noTrunc := cmd.Bool("notrunc", false, "Don't truncate output")
	nLatest := cmd.Bool("l", false, "Show only the latest created container, include non-running ones.")
	since := cmd.String("sinceId", "", "Show only containers created since Id, include non-running ones.")
	before := cmd.String("beforeId", "", "Show only container created before Id, include non-running ones.")
	last := cmd.Int("n", -1, "Show n last created containers, include non-running ones.")

	if err := cmd.Parse(args); err != nil {
		return nil
	}
	v := url.Values{}
	if *last == -1 && *nLatest {
		*last = 1
	}
	if *all {
		v.Set("all", "1")
	}
	if *last != -1 {
		v.Set("limit", strconv.Itoa(*last))
	}
	if *since != "" {
		v.Set("since", *since)
	}
	if *before != "" {
		v.Set("before", *before)
	}

	body, _, err := cli.call("GET", "/containers/ps?"+v.Encode(), nil)
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
			if *noTrunc {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s ago\t%s\n", out.Id, out.Image, out.Command, out.Status, utils.HumanDuration(time.Now().Sub(time.Unix(out.Created, 0))), out.Ports)
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s ago\t%s\n", utils.TruncateId(out.Id), out.Image, utils.Trunc(out.Command, 20), out.Status, utils.HumanDuration(time.Now().Sub(time.Unix(out.Created, 0))), out.Ports)
			}
		} else {
			if *noTrunc {
				fmt.Fprintln(w, out.Id)
			} else {
				fmt.Fprintln(w, utils.TruncateId(out.Id))
			}
		}
	}

	if !*quiet {
		w.Flush()
	}
	return nil
}

func (cli *DockerCli) CmdCommit(args ...string) error {
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
	body, _, err := cli.call("POST", "/commit?"+v.Encode(), config)
	if err != nil {
		return err
	}

	apiId := &ApiId{}
	err = json.Unmarshal(body, apiId)
	if err != nil {
		return err
	}

	fmt.Println(apiId.Id)
	return nil
}

func (cli *DockerCli) CmdExport(args ...string) error {
	cmd := Subcmd("export", "CONTAINER", "Export the contents of a filesystem as a tar archive")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	if err := cli.stream("GET", "/containers/"+cmd.Arg(0)+"/export", nil, os.Stdout); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) CmdDiff(args ...string) error {
	cmd := Subcmd("diff", "CONTAINER", "Inspect changes on a container's filesystem")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	body, _, err := cli.call("GET", "/containers/"+cmd.Arg(0)+"/changes", nil)
	if err != nil {
		return err
	}

	changes := []Change{}
	err = json.Unmarshal(body, &changes)
	if err != nil {
		return err
	}
	for _, change := range changes {
		fmt.Println(change.String())
	}
	return nil
}

func (cli *DockerCli) CmdLogs(args ...string) error {
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

	if err := cli.hijack("POST", "/containers/"+cmd.Arg(0)+"/attach?"+v.Encode(), false); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) CmdAttach(args ...string) error {
	cmd := Subcmd("attach", "CONTAINER", "Attach to a running container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	body, _, err := cli.call("GET", "/containers/"+cmd.Arg(0)+"/json", nil)
	if err != nil {
		return err
	}

	container := &Container{}
	err = json.Unmarshal(body, container)
	if err != nil {
		return err
	}

	v := url.Values{}
	v.Set("stream", "1")
	v.Set("stdout", "1")
	v.Set("stderr", "1")
	v.Set("stdin", "1")

	if err := cli.hijack("POST", "/containers/"+cmd.Arg(0)+"/attach?"+v.Encode(), container.Config.Tty); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) CmdSearch(args ...string) error {
	cmd := Subcmd("search", "NAME", "Search the docker index for images")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	v := url.Values{}
	v.Set("term", cmd.Arg(0))
	body, _, err := cli.call("GET", "/images/search?"+v.Encode(), nil)
	if err != nil {
		return err
	}

	outs := []ApiSearch{}
	err = json.Unmarshal(body, &outs)
	if err != nil {
		return err
	}
	fmt.Printf("Found %d results matching your query (\"%s\")\n", len(outs), cmd.Arg(0))
	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	fmt.Fprintf(w, "NAME\tDESCRIPTION\n")
	for _, out := range outs {
		fmt.Fprintf(w, "%s\t%s\n", out.Name, out.Description)
	}
	w.Flush()
	return nil
}

// Ports type - Used to parse multiple -p flags
type ports []int

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

func (cli *DockerCli) CmdTag(args ...string) error {
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

	if _, _, err := cli.call("POST", "/images/"+cmd.Arg(0)+"/tag?"+v.Encode(), nil); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) CmdRun(args ...string) error {
	config, cmd, err := ParseRun(args, nil)
	if err != nil {
		return err
	}
	if config.Image == "" {
		cmd.Usage()
		return nil
	}

	//create the container
	body, statusCode, err := cli.call("POST", "/containers/create", config)
	//if image not found try to pull it
	if statusCode == 404 {
		v := url.Values{}
		v.Set("fromImage", config.Image)
		err = cli.stream("POST", "/images/create?"+v.Encode(), nil, os.Stderr)
		if err != nil {
			return err
		}
		body, _, err = cli.call("POST", "/containers/create", config)
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}

	out := &ApiRun{}
	err = json.Unmarshal(body, out)
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
	_, _, err = cli.call("POST", "/containers/"+out.Id+"/start", nil)
	if err != nil {
		return err
	}

	if config.AttachStdin || config.AttachStdout || config.AttachStderr {
		if err := cli.hijack("POST", "/containers/"+out.Id+"/attach?"+v.Encode(), config.Tty); err != nil {
			return err
		}
	}
	if !config.AttachStdout && !config.AttachStderr {
		fmt.Println(out.Id)
	}
	return nil
}

func (cli *DockerCli) call(method, path string, data interface{}) ([]byte, int, error) {
	var params io.Reader
	if data != nil {
		buf, err := json.Marshal(data)
		if err != nil {
			return nil, -1, err
		}
		params = bytes.NewBuffer(buf)
	}

	req, err := http.NewRequest(method, fmt.Sprintf("http://%s:%d", cli.host, cli.port)+path, params)
	if err != nil {
		return nil, -1, err
	}
	req.Header.Set("User-Agent", "Docker-Client/"+VERSION)
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	} else if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, -1, fmt.Errorf("Can't connect to docker daemon. Is 'docker -d' running on this host?")
		}
		return nil, -1, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, -1, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, resp.StatusCode, fmt.Errorf("error: %s", body)
	}
	return body, resp.StatusCode, nil
}

func (cli *DockerCli) stream(method, path string, in io.Reader, out io.Writer) error {
	if (method == "POST" || method == "PUT") && in == nil {
		in = bytes.NewReader([]byte{})
	}
	req, err := http.NewRequest(method, fmt.Sprintf("http://%s:%d%s", cli.host, cli.port, path), in)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Docker-Client/"+VERSION)
	if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return fmt.Errorf("Can't connect to docker daemon. Is 'docker -d' running on this host?")
		}
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("error: %s", body)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) hijack(method, path string, setRawTerminal bool) error {
	req, err := http.NewRequest(method, path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "plain/text")
	dial, err := net.Dial("tcp", fmt.Sprintf("%s:%d", cli.host, cli.port))
	if err != nil {
		return err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	clientconn.Do(req)
	defer clientconn.Close()

	rwc, br := clientconn.Hijack()
	defer rwc.Close()

	receiveStdout := utils.Go(func() error {
		_, err := io.Copy(os.Stdout, br)
		return err
	})

	if setRawTerminal && term.IsTerminal(int(os.Stdin.Fd())) && os.Getenv("NORAW") == "" {
		if oldState, err := term.SetRawTerminal(); err != nil {
			return err
		} else {
			defer term.RestoreTerminal(oldState)
		}
	}

	sendStdin := utils.Go(func() error {
		_, err := io.Copy(rwc, os.Stdin)
		if err := rwc.(*net.TCPConn).CloseWrite(); err != nil {
			fmt.Fprintf(os.Stderr, "Couldn't send EOF: %s\n", err)
		}
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

func NewDockerCli(host string, port int) *DockerCli {
	return &DockerCli{host, port}
}

type DockerCli struct {
	host string
	port int
}
