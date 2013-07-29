package docker

import (
	"archive/tar"
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
	"os/signal"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"
	"unicode"
)

const VERSION = "0.5.0-dev"

var (
	GITCOMMIT string
)

func (cli *DockerCli) getMethod(name string) (reflect.Method, bool) {
	methodName := "Cmd" + strings.ToUpper(name[:1]) + strings.ToLower(name[1:])
	return reflect.TypeOf(cli).MethodByName(methodName)
}

func ParseCommands(proto, addr string, args ...string) error {
	cli := NewDockerCli(os.Stdin, os.Stdout, os.Stderr, proto, addr)

	if len(args) > 0 {
		method, exists := cli.getMethod(args[0])
		if !exists {
			fmt.Println("Error: Command not found:", args[0])
			return cli.CmdHelp(args[1:]...)
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
	if len(args) > 0 {
		method, exists := cli.getMethod(args[0])
		if !exists {
			fmt.Fprintf(cli.err, "Error: Command not found: %s\n", args[0])
		} else {
			method.Func.CallSlice([]reflect.Value{
				reflect.ValueOf(cli),
				reflect.ValueOf([]string{"--help"}),
			})[0].Interface()
			return nil
		}
	}
	help := fmt.Sprintf("Usage: docker [OPTIONS] COMMAND [arg...]\n  -H=[tcp://%s:%d]: tcp://host:port to bind/connect to or unix://path/to/socket to use\n\nA self-sufficient runtime for linux containers.\n\nCommands:\n", DEFAULTHTTPHOST, DEFAULTHTTPPORT)
	for _, command := range [][]string{
		{"attach", "Attach to a running container"},
		{"build", "Build a container from a Dockerfile"},
		{"commit", "Create a new image from a container's changes"},
		{"diff", "Inspect changes on a container's filesystem"},
		{"events", "Get real time events from the server"},
		{"export", "Stream the contents of a container as a tar archive"},
		{"history", "Show the history of an image"},
		{"images", "List images"},
		{"import", "Create a new filesystem image from the contents of a tarball"},
		{"info", "Display system-wide information"},
		{"insert", "Insert a file in an image"},
		{"inspect", "Return low-level information on a container"},
		{"kill", "Kill a running container"},
		{"login", "Register or Login to the docker registry server"},
		{"logs", "Fetch the logs of a container"},
		{"port", "Lookup the public-facing port which is NAT-ed to PRIVATE_PORT"},
		{"top", "Lookup the running processes of a container"},
		{"ps", "List containers"},
		{"pull", "Pull an image or a repository from the docker registry server"},
		{"push", "Push an image or a repository to the docker registry server"},
		{"restart", "Restart a running container"},
		{"rm", "Remove one or more containers"},
		{"rmi", "Remove one or more images"},
		{"run", "Run a command in a new container"},
		{"search", "Search for an image in the docker index"},
		{"start", "Start a stopped container"},
		{"stop", "Stop a running container"},
		{"tag", "Tag an image into a repository"},
		{"version", "Show the docker version information"},
		{"wait", "Block until a container stops, then print its exit code"},
	} {
		help += fmt.Sprintf("    %-10.10s%s\n", command[0], command[1])
	}
	fmt.Fprintf(cli.err, "%s\n", help)
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

	if err := cli.stream("POST", "/images/"+cmd.Arg(0)+"/insert?"+v.Encode(), nil, cli.out); err != nil {
		return err
	}
	return nil
}

// mkBuildContext returns an archive of an empty context with the contents
// of `dockerfile` at the path ./Dockerfile
func mkBuildContext(dockerfile string, files [][2]string) (Archive, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	files = append(files, [2]string{"Dockerfile", dockerfile})
	for _, file := range files {
		name, content := file[0], file[1]
		hdr := &tar.Header{
			Name: name,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf, nil
}

func (cli *DockerCli) CmdBuild(args ...string) error {
	cmd := Subcmd("build", "[OPTIONS] PATH | URL | -", "Build a new container image from the source code at PATH")
	tag := cmd.String("t", "", "Tag to be applied to the resulting image in case of success")
	suppressOutput := cmd.Bool("q", false, "Suppress verbose build output")

	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	var (
		context  Archive
		isRemote bool
		err      error
	)

	if cmd.Arg(0) == "-" {
		// As a special case, 'docker build -' will build from an empty context with the
		// contents of stdin as a Dockerfile
		dockerfile, err := ioutil.ReadAll(cli.in)
		if err != nil {
			return err
		}
		context, err = mkBuildContext(string(dockerfile), nil)
	} else if utils.IsURL(cmd.Arg(0)) || utils.IsGIT(cmd.Arg(0)) {
		isRemote = true
	} else {
		if _, err := os.Stat(cmd.Arg(0)); err != nil {
			return err
		}
		context, err = Tar(cmd.Arg(0), Uncompressed)
	}
	var body io.Reader
	// Setup an upload progress bar
	// FIXME: ProgressReader shouldn't be this annoyning to use
	if context != nil {
		sf := utils.NewStreamFormatter(false)
		body = utils.ProgressReader(ioutil.NopCloser(context), 0, cli.err, sf.FormatProgress("Uploading context", "%v bytes%0.0s%0.0s"), sf)
	}
	// Upload the build context
	v := &url.Values{}
	v.Set("t", *tag)

	if *suppressOutput {
		v.Set("q", "1")
	}
	if isRemote {
		v.Set("remote", cmd.Arg(0))
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("/v%g/build?%s", APIVERSION, v.Encode()), body)
	if err != nil {
		return err
	}
	if context != nil {
		req.Header.Set("Content-Type", "application/tar")
	}
	dial, err := net.Dial(cli.proto, cli.addr)
	if err != nil {
		return err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	resp, err := clientconn.Do(req)
	defer clientconn.Close()
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Check for errors
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if len(body) == 0 {
			return fmt.Errorf("Error: %s", http.StatusText(resp.StatusCode))
		}
		return fmt.Errorf("Error: %s", body)
	}

	// Output the result
	if _, err := io.Copy(cli.out, resp.Body); err != nil {
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

	cmd := Subcmd("login", "[OPTIONS]", "Register or Login to the docker registry server")
	flUsername := cmd.String("u", "", "username")
	flPassword := cmd.String("p", "", "password")
	flEmail := cmd.String("e", "", "email")
	err := cmd.Parse(args)
	if err != nil {
		return nil
	}

	var oldState *term.State
	if *flUsername == "" || *flPassword == "" || *flEmail == "" {
		oldState, err = term.SetRawTerminal(cli.terminalFd)
		if err != nil {
			return err
		}
		defer term.RestoreTerminal(cli.terminalFd, oldState)
	}

	var (
		username string
		password string
		email    string
	)

	authconfig, ok := cli.configFile.Configs[auth.IndexServerAddress()]
	if !ok {
		authconfig = auth.AuthConfig{}
	}

	if *flUsername == "" {
		fmt.Fprintf(cli.out, "Username (%s): ", authconfig.Username)
		username = readAndEchoString(cli.in, cli.out)
		if username == "" {
			username = authconfig.Username
		}
	} else {
		username = *flUsername
	}
	if username != authconfig.Username {
		if *flPassword == "" {
			fmt.Fprintf(cli.out, "Password: ")
			password = readString(cli.in, cli.out)
			if password == "" {
				return fmt.Errorf("Error : Password Required")
			}
		} else {
			password = *flPassword
		}

		if *flEmail == "" {
			fmt.Fprintf(cli.out, "Email (%s): ", authconfig.Email)
			email = readAndEchoString(cli.in, cli.out)
			if email == "" {
				email = authconfig.Email
			}
		} else {
			email = *flEmail
		}
	} else {
		password = authconfig.Password
		email = authconfig.Email
	}
	if oldState != nil {
		term.RestoreTerminal(cli.terminalFd, oldState)
	}
	authconfig.Username = username
	authconfig.Password = password
	authconfig.Email = email
	cli.configFile.Configs[auth.IndexServerAddress()] = authconfig

	body, statusCode, err := cli.call("POST", "/auth", cli.configFile.Configs[auth.IndexServerAddress()])
	if statusCode == 401 {
		delete(cli.configFile.Configs, auth.IndexServerAddress())
		auth.SaveConfig(cli.configFile)
		return err
	}
	if err != nil {
		return err
	}

	var out2 APIAuth
	err = json.Unmarshal(body, &out2)
	if err != nil {
		cli.configFile, _ = auth.LoadConfig(os.Getenv("HOME"))
		return err
	}
	auth.SaveConfig(cli.configFile)
	if out2.Status != "" {
		fmt.Fprintf(cli.out, "%s\n", out2.Status)
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
			fmt.Fprintf(cli.err, "%s", err)
		} else {
			var out APIWait
			err = json.Unmarshal(body, &out)
			if err != nil {
				return err
			}
			fmt.Fprintf(cli.out, "%d\n", out.StatusCode)
		}
	}
	return nil
}

// 'docker version': show version information
func (cli *DockerCli) CmdVersion(args ...string) error {
	cmd := Subcmd("version", "", "Show the docker version information.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() > 0 {
		cmd.Usage()
		return nil
	}

	body, _, err := cli.call("GET", "/version", nil)
	if err != nil {
		return err
	}

	var out APIVersion
	err = json.Unmarshal(body, &out)
	if err != nil {
		utils.Debugf("Error unmarshal: body: %s, err: %s\n", body, err)
		return err
	}
	fmt.Fprintf(cli.out, "Client version: %s\n", VERSION)
	fmt.Fprintf(cli.out, "Server version: %s\n", out.Version)
	if out.GitCommit != "" {
		fmt.Fprintf(cli.out, "Git commit: %s\n", out.GitCommit)
	}
	if out.GoVersion != "" {
		fmt.Fprintf(cli.out, "Go version: %s\n", out.GoVersion)
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

	var out APIInfo
	if err := json.Unmarshal(body, &out); err != nil {
		return err
	}

	fmt.Fprintf(cli.out, "Containers: %d\n", out.Containers)
	fmt.Fprintf(cli.out, "Images: %d\n", out.Images)
	if out.Debug || os.Getenv("DEBUG") != "" {
		fmt.Fprintf(cli.out, "Debug mode (server): %v\n", out.Debug)
		fmt.Fprintf(cli.out, "Debug mode (client): %v\n", os.Getenv("DEBUG") != "")
		fmt.Fprintf(cli.out, "Fds: %d\n", out.NFd)
		fmt.Fprintf(cli.out, "Goroutines: %d\n", out.NGoroutines)
		fmt.Fprintf(cli.out, "LXC Version: %s\n", out.LXCVersion)
		fmt.Fprintf(cli.out, "EventsListeners: %d\n", out.NEventsListener)
		fmt.Fprintf(cli.out, "Kernel Version: %s\n", out.KernelVersion)
	}
	if !out.MemoryLimit {
		fmt.Fprintf(cli.err, "WARNING: No memory limit support\n")
	}
	if !out.SwapLimit {
		fmt.Fprintf(cli.err, "WARNING: No swap limit support\n")
	}
	return nil
}

func (cli *DockerCli) CmdStop(args ...string) error {
	cmd := Subcmd("stop", "[OPTIONS] CONTAINER [CONTAINER...]", "Stop a running container")
	nSeconds := cmd.Int("t", 10, "Number of seconds to wait for the container to stop before killing it.")
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
			fmt.Fprintf(cli.err, "%s\n", err)
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return nil
}

func (cli *DockerCli) CmdRestart(args ...string) error {
	cmd := Subcmd("restart", "[OPTIONS] CONTAINER [CONTAINER...]", "Restart a running container")
	nSeconds := cmd.Int("t", 10, "Number of seconds to try to stop for before killing the container. Once killed it will then be restarted. Default=10")
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
			fmt.Fprintf(cli.err, "%s\n", err)
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
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
			fmt.Fprintf(cli.err, "%s\n", err)
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return nil
}

func (cli *DockerCli) CmdInspect(args ...string) error {
	cmd := Subcmd("inspect", "CONTAINER|IMAGE [CONTAINER|IMAGE...]", "Return low-level information on a container/image")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	fmt.Fprintf(cli.out, "[")
	for i, name := range args {
		if i > 0 {
			fmt.Fprintf(cli.out, ",")
		}
		obj, _, err := cli.call("GET", "/containers/"+name+"/json", nil)
		if err != nil {
			obj, _, err = cli.call("GET", "/images/"+name+"/json", nil)
			if err != nil {
				fmt.Fprintf(cli.err, "%s\n", err)
				continue
			}
		}

		indented := new(bytes.Buffer)
		if err = json.Indent(indented, obj, "", "    "); err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			continue
		}
		if _, err := io.Copy(cli.out, indented); err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
		}
	}
	fmt.Fprintf(cli.out, "]")
	return nil
}

func (cli *DockerCli) CmdTop(args ...string) error {
	cmd := Subcmd("top", "CONTAINER", "Lookup the running processes of a container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}
	body, _, err := cli.call("GET", "/containers/"+cmd.Arg(0)+"/top", nil)
	if err != nil {
		return err
	}
	var procs []APITop
	err = json.Unmarshal(body, &procs)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	fmt.Fprintln(w, "PID\tTTY\tTIME\tCMD")
	for _, proc := range procs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", proc.PID, proc.Tty, proc.Time, proc.Cmd)
	}
	w.Flush()
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

	port := cmd.Arg(1)
	proto := "Tcp"
	parts := strings.SplitN(port, "/", 2)
	if len(parts) == 2 && len(parts[1]) != 0 {
		port = parts[0]
		proto = strings.ToUpper(parts[1][:1]) + strings.ToLower(parts[1][1:])
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

	if frontend, exists := out.NetworkSettings.PortMapping[proto][port]; exists {
		fmt.Fprintf(cli.out, "%s\n", frontend)
	} else {
		return fmt.Errorf("Error: No private port '%s' allocated on %s", cmd.Arg(1), cmd.Arg(0))
	}
	return nil
}

// 'docker rmi IMAGE' removes all images with the name IMAGE
func (cli *DockerCli) CmdRmi(args ...string) error {
	cmd := Subcmd("rmi", "IMAGE [IMAGE...]", "Remove one or more images")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range cmd.Args() {
		body, _, err := cli.call("DELETE", "/images/"+name, nil)
		if err != nil {
			fmt.Fprintf(cli.err, "%s", err)
		} else {
			var outs []APIRmi
			err = json.Unmarshal(body, &outs)
			if err != nil {
				return err
			}
			for _, out := range outs {
				if out.Deleted != "" {
					fmt.Fprintf(cli.out, "Deleted: %s\n", out.Deleted)
				} else {
					fmt.Fprintf(cli.out, "Untagged: %s\n", out.Untagged)
				}
			}
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

	var outs []APIHistory
	err = json.Unmarshal(body, &outs)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tCREATED\tCREATED BY")

	for _, out := range outs {
		if out.Tags != nil {
			out.ID = out.Tags[0]
		}
		fmt.Fprintf(w, "%s \t%s ago\t%s\n", out.ID, utils.HumanDuration(time.Now().Sub(time.Unix(out.Created, 0))), out.CreatedBy)
	}
	w.Flush()
	return nil
}

func (cli *DockerCli) CmdRm(args ...string) error {
	cmd := Subcmd("rm", "[OPTIONS] CONTAINER [CONTAINER...]", "Remove one or more containers")
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
			fmt.Fprintf(cli.err, "%s\n", err)
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
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
			fmt.Fprintf(cli.err, "%s\n", err)
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return nil
}

func (cli *DockerCli) CmdImport(args ...string) error {
	cmd := Subcmd("import", "URL|- [REPOSITORY [TAG]]", "Create a new filesystem image from the contents of a tarball(.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz).")

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

	err := cli.stream("POST", "/images/create?"+v.Encode(), cli.in, cli.out)
	if err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) CmdPush(args ...string) error {
	cmd := Subcmd("push", "NAME", "Push an image or a repository to the registry")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	name := cmd.Arg(0)

	if name == "" {
		cmd.Usage()
		return nil
	}

	if err := cli.checkIfLogged("push"); err != nil {
		return err
	}

	// If we're not using a custom registry, we know the restrictions
	// applied to repository names and can warn the user in advance.
	// Custom repositories can have different rules, and we must also
	// allow pushing by image ID.
	if len(strings.SplitN(name, "/", 2)) == 1 {
		return fmt.Errorf("Impossible to push a \"root\" repository. Please rename your repository in <user>/<repo> (ex: %s/%s)", cli.configFile.Configs[auth.IndexServerAddress()].Username, name)
	}

	buf, err := json.Marshal(cli.configFile.Configs[auth.IndexServerAddress()])
	if err != nil {
		return err
	}

	v := url.Values{}
	if err := cli.stream("POST", "/images/"+name+"/push?"+v.Encode(), bytes.NewBuffer(buf), cli.out); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) CmdPull(args ...string) error {
	cmd := Subcmd("pull", "NAME", "Pull an image or a repository from the registry")
	tag := cmd.String("t", "", "Download tagged image in repository")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	remote, parsedTag := utils.ParseRepositoryTag(cmd.Arg(0))
	if *tag == "" {
		*tag = parsedTag
	}

	v := url.Values{}
	v.Set("fromImage", remote)
	v.Set("tag", *tag)

	if err := cli.stream("POST", "/images/create?"+v.Encode(), nil, cli.out); err != nil {
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
		fmt.Fprintf(cli.out, "%s", body)
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

		var outs []APIImages
		err = json.Unmarshal(body, &outs)
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
		if !*quiet {
			fmt.Fprintln(w, "REPOSITORY\tTAG\tID\tCREATED\tSIZE")
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
					fmt.Fprintf(w, "%s\t", out.ID)
				} else {
					fmt.Fprintf(w, "%s\t", utils.TruncateID(out.ID))
				}
				fmt.Fprintf(w, "%s ago\t", utils.HumanDuration(time.Now().Sub(time.Unix(out.Created, 0))))
				if out.VirtualSize > 0 {
					fmt.Fprintf(w, "%s (virtual %s)\n", utils.HumanSize(out.Size), utils.HumanSize(out.VirtualSize))
				} else {
					fmt.Fprintf(w, "%s\n", utils.HumanSize(out.Size))
				}
			} else {
				if *noTrunc {
					fmt.Fprintln(w, out.ID)
				} else {
					fmt.Fprintln(w, utils.TruncateID(out.ID))
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
	size := cmd.Bool("s", false, "Display sizes")
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
	if *size {
		v.Set("size", "1")
	}

	body, _, err := cli.call("GET", "/containers/json?"+v.Encode(), nil)
	if err != nil {
		return err
	}

	var outs []APIContainers
	err = json.Unmarshal(body, &outs)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	if !*quiet {
		fmt.Fprint(w, "ID\tIMAGE\tCOMMAND\tCREATED\tSTATUS\tPORTS")
		if *size {
			fmt.Fprintln(w, "\tSIZE")
		} else {
			fmt.Fprint(w, "\n")
		}
	}

	for _, out := range outs {
		if !*quiet {
			if *noTrunc {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s ago\t%s\t%s\t", out.ID, out.Image, out.Command, utils.HumanDuration(time.Now().Sub(time.Unix(out.Created, 0))), out.Status, out.Ports)
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s ago\t%s\t%s\t", utils.TruncateID(out.ID), out.Image, utils.Trunc(out.Command, 20), utils.HumanDuration(time.Now().Sub(time.Unix(out.Created, 0))), out.Status, out.Ports)
			}
			if *size {
				if out.SizeRootFs > 0 {
					fmt.Fprintf(w, "%s (virtual %s)\n", utils.HumanSize(out.SizeRw), utils.HumanSize(out.SizeRootFs))
				} else {
					fmt.Fprintf(w, "%s\n", utils.HumanSize(out.SizeRw))
				}
			} else {
				fmt.Fprint(w, "\n")
			}
		} else {
			if *noTrunc {
				fmt.Fprintln(w, out.ID)
			} else {
				fmt.Fprintln(w, utils.TruncateID(out.ID))
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

	apiID := &APIID{}
	err = json.Unmarshal(body, apiID)
	if err != nil {
		return err
	}

	fmt.Fprintf(cli.out, "%s\n", apiID.ID)
	return nil
}

func (cli *DockerCli) CmdEvents(args ...string) error {
	cmd := Subcmd("events", "[OPTIONS]", "Get real time events from the server")
	since := cmd.String("since", "", "Show events previously created (used for polling).")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() != 0 {
		cmd.Usage()
		return nil
	}

	v := url.Values{}
	if *since != "" {
		v.Set("since", *since)
	}

	if err := cli.stream("GET", "/events?"+v.Encode(), nil, cli.out); err != nil {
		return err
	}
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

	if err := cli.stream("GET", "/containers/"+cmd.Arg(0)+"/export", nil, cli.out); err != nil {
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
		fmt.Fprintf(cli.out, "%s\n", change.String())
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

	if err := cli.hijack("POST", "/containers/"+cmd.Arg(0)+"/attach?logs=1&stdout=1&stderr=1", false, nil, cli.out); err != nil {
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

	if !container.State.Running {
		return fmt.Errorf("Impossible to attach to a stopped container, start it first")
	}

	if container.Config.Tty {
		if err := cli.monitorTtySize(cmd.Arg(0)); err != nil {
			return err
		}
	}

	v := url.Values{}
	v.Set("stream", "1")
	v.Set("stdin", "1")
	v.Set("stdout", "1")
	v.Set("stderr", "1")

	if err := cli.hijack("POST", "/containers/"+cmd.Arg(0)+"/attach?"+v.Encode(), container.Config.Tty, cli.in, cli.out); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) CmdSearch(args ...string) error {
	cmd := Subcmd("search", "NAME", "Search the docker index for images")
	noTrunc := cmd.Bool("notrunc", false, "Don't truncate output")
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

	outs := []APISearch{}
	err = json.Unmarshal(body, &outs)
	if err != nil {
		return err
	}
	fmt.Fprintf(cli.out, "Found %d results matching your query (\"%s\")\n", len(outs), cmd.Arg(0))
	w := tabwriter.NewWriter(cli.out, 33, 1, 3, ' ', 0)
	fmt.Fprintf(w, "NAME\tDESCRIPTION\n")
	_, width := cli.getTtySize()
	if width == 0 {
		width = 45
	} else {
		width = width - 33 //remote the first column
	}
	for _, out := range outs {
		desc := strings.Replace(out.Description, "\n", " ", -1)
		desc = strings.Replace(desc, "\r", " ", -1)
		if !*noTrunc && len(desc) > width {
			desc = utils.Trunc(desc, width-3) + "..."
		}
		fmt.Fprintf(w, "%s\t%s\n", out.Name, desc)
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
	var containerPath string

	splited := strings.SplitN(val, ":", 2)
	if len(splited) == 1 {
		containerPath = splited[0]
		val = filepath.Clean(splited[0])
	} else {
		containerPath = splited[1]
		val = fmt.Sprintf("%s:%s", splited[0], filepath.Clean(splited[1]))
	}

	if !filepath.IsAbs(containerPath) {
		utils.Debugf("%s is not an absolute path", containerPath)
		return fmt.Errorf("%s is not an absolute path", containerPath)
	}
	opts[val] = struct{}{}
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
	config, hostConfig, cmd, err := ParseRun(args, nil)
	if err != nil {
		return err
	}
	if config.Image == "" {
		cmd.Usage()
		return nil
	}

	var containerIDFile *os.File
	if len(hostConfig.ContainerIDFile) > 0 {
		if _, err := ioutil.ReadFile(hostConfig.ContainerIDFile); err == nil {
			return fmt.Errorf("cid file found, make sure the other container isn't running or delete %s", hostConfig.ContainerIDFile)
		}
		containerIDFile, err = os.Create(hostConfig.ContainerIDFile)
		if err != nil {
			return fmt.Errorf("failed to create the container ID file: %s", err)
		}
		defer containerIDFile.Close()
	}

	//create the container
	body, statusCode, err := cli.call("POST", "/containers/create", config)
	//if image not found try to pull it
	if statusCode == 404 {
		v := url.Values{}
		repos, tag := utils.ParseRepositoryTag(config.Image)
		v.Set("fromImage", repos)
		v.Set("tag", tag)
		err = cli.stream("POST", "/images/create?"+v.Encode(), nil, cli.err)
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

	runResult := &APIRun{}
	err = json.Unmarshal(body, runResult)
	if err != nil {
		return err
	}

	for _, warning := range runResult.Warnings {
		fmt.Fprintf(cli.err, "WARNING: %s\n", warning)
	}
	if len(hostConfig.ContainerIDFile) > 0 {
		if _, err = containerIDFile.WriteString(runResult.ID); err != nil {
			return fmt.Errorf("failed to write the container ID to the file: %s", err)
		}
	}

	//start the container
	if _, _, err = cli.call("POST", "/containers/"+runResult.ID+"/start", hostConfig); err != nil {
		return err
	}

	var wait chan struct{}

	if !config.AttachStdout && !config.AttachStderr {
		// Make this asynchrone in order to let the client write to stdin before having to read the ID
		wait = make(chan struct{})
		go func() {
			defer close(wait)
			fmt.Fprintf(cli.out, "%s\n", runResult.ID)
		}()
	}

	if config.AttachStdin || config.AttachStdout || config.AttachStderr {
		if config.Tty {
			if err := cli.monitorTtySize(runResult.ID); err != nil {
				utils.Debugf("Error monitoring TTY size: %s\n", err)
			}
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

		if err := cli.hijack("POST", "/containers/"+runResult.ID+"/attach?"+v.Encode(), config.Tty, cli.in, cli.out); err != nil {
			utils.Debugf("Error hijack: %s", err)
			return err
		}
	}

	if !config.AttachStdout && !config.AttachStderr {
		<-wait
	}
	return nil
}

func (cli *DockerCli) checkIfLogged(action string) error {
	// If condition AND the login failed
	if cli.configFile.Configs[auth.IndexServerAddress()].Username == "" {
		if err := cli.CmdLogin(""); err != nil {
			return err
		}
		if cli.configFile.Configs[auth.IndexServerAddress()].Username == "" {
			return fmt.Errorf("Please login prior to %s. ('docker login')", action)
		}
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

	req, err := http.NewRequest(method, fmt.Sprintf("/v%g%s", APIVERSION, path), params)
	if err != nil {
		return nil, -1, err
	}
	req.Header.Set("User-Agent", "Docker-Client/"+VERSION)
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	} else if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}
	dial, err := net.Dial(cli.proto, cli.addr)
	if err != nil {
		return nil, -1, err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	resp, err := clientconn.Do(req)
	defer clientconn.Close()
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
		if len(body) == 0 {
			return nil, resp.StatusCode, fmt.Errorf("Error: %s", http.StatusText(resp.StatusCode))
		}
		return nil, resp.StatusCode, fmt.Errorf("Error: %s", body)
	}
	return body, resp.StatusCode, nil
}

func (cli *DockerCli) stream(method, path string, in io.Reader, out io.Writer) error {
	if (method == "POST" || method == "PUT") && in == nil {
		in = bytes.NewReader([]byte{})
	}
	req, err := http.NewRequest(method, fmt.Sprintf("/v%g%s", APIVERSION, path), in)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Docker-Client/"+VERSION)
	if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}
	dial, err := net.Dial(cli.proto, cli.addr)
	if err != nil {
		return err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	resp, err := clientconn.Do(req)
	defer clientconn.Close()
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
		if len(body) == 0 {
			return fmt.Errorf("Error :%s", http.StatusText(resp.StatusCode))
		}
		return fmt.Errorf("Error: %s", body)
	}

	if resp.Header.Get("Content-Type") == "application/json" {
		dec := json.NewDecoder(resp.Body)
		for {
			var jm utils.JSONMessage
			if err := dec.Decode(&jm); err == io.EOF {
				break
			} else if err != nil {
				return err
			}
			jm.Display(out)
		}
	} else {
		if _, err := io.Copy(out, resp.Body); err != nil {
			return err
		}
	}
	return nil
}

func (cli *DockerCli) hijack(method, path string, setRawTerminal bool, in io.ReadCloser, out io.Writer) error {

	req, err := http.NewRequest(method, fmt.Sprintf("/v%g%s", APIVERSION, path), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Docker-Client/"+VERSION)
	req.Header.Set("Content-Type", "plain/text")

	dial, err := net.Dial(cli.proto, cli.addr)
	if err != nil {
		return err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	defer clientconn.Close()

	// Server hijacks the connection, error 'connection closed' expected
	clientconn.Do(req)

	rwc, br := clientconn.Hijack()
	defer rwc.Close()

	receiveStdout := utils.Go(func() error {
		_, err := io.Copy(out, br)
		utils.Debugf("[hijack] End of stdout")
		return err
	})

	if in != nil && setRawTerminal && cli.isTerminal && os.Getenv("NORAW") == "" {
		oldState, err := term.SetRawTerminal(cli.terminalFd)
		if err != nil {
			return err
		}
		defer term.RestoreTerminal(cli.terminalFd, oldState)
	}

	sendStdin := utils.Go(func() error {
		if in != nil {
			io.Copy(rwc, in)
			utils.Debugf("[hijack] End of stdin")
		}
		if tcpc, ok := rwc.(*net.TCPConn); ok {
			if err := tcpc.CloseWrite(); err != nil {
				utils.Debugf("Couldn't send EOF: %s\n", err)
			}
		} else if unixc, ok := rwc.(*net.UnixConn); ok {
			if err := unixc.CloseWrite(); err != nil {
				utils.Debugf("Couldn't send EOF: %s\n", err)
			}
		}
		// Discard errors due to pipe interruption
		return nil
	})

	if err := <-receiveStdout; err != nil {
		utils.Debugf("Error receiveStdout: %s", err)
		return err
	}

	if !cli.isTerminal {
		if err := <-sendStdin; err != nil {
			utils.Debugf("Error sendStdin: %s", err)
			return err
		}
	}
	return nil

}

func (cli *DockerCli) getTtySize() (int, int) {
	if !cli.isTerminal {
		return 0, 0
	}
	ws, err := term.GetWinsize(cli.terminalFd)
	if err != nil {
		utils.Debugf("Error getting size: %s", err)
		if ws == nil {
			return 0, 0
		}
	}
	return int(ws.Height), int(ws.Width)
}

func (cli *DockerCli) resizeTty(id string) {
	height, width := cli.getTtySize()
	if height == 0 && width == 0 {
		return
	}
	v := url.Values{}
	v.Set("h", strconv.Itoa(height))
	v.Set("w", strconv.Itoa(width))
	if _, _, err := cli.call("POST", "/containers/"+id+"/resize?"+v.Encode(), nil); err != nil {
		utils.Debugf("Error resize: %s", err)
	}
}

func (cli *DockerCli) monitorTtySize(id string) error {
	if !cli.isTerminal {
		return fmt.Errorf("Impossible to monitor size on non-tty")
	}
	cli.resizeTty(id)

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGWINCH)
	go func() {
		<-sigchan
		cli.resizeTty(id)
	}()
	return nil
}

func Subcmd(name, signature, description string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.Usage = func() {
		// FIXME: use custom stdout or return error
		fmt.Fprintf(os.Stdout, "\nUsage: docker %s %s\n\n%s\n\n", name, signature, description)
		flags.PrintDefaults()
	}
	return flags
}

func NewDockerCli(in io.ReadCloser, out, err io.Writer, proto, addr string) *DockerCli {
	var (
		isTerminal = false
		terminalFd uintptr
	)

	if in != nil {
		if file, ok := in.(*os.File); ok {
			terminalFd = file.Fd()
			isTerminal = term.IsTerminal(terminalFd)
		}
	}

	if err == nil {
		err = out
	}

	configFile, _ := auth.LoadConfig(os.Getenv("HOME"))
	return &DockerCli{
		proto:      proto,
		addr:       addr,
		configFile: configFile,
		in:         in,
		out:        out,
		err:        err,
		isTerminal: isTerminal,
		terminalFd: terminalFd,
	}
}

type DockerCli struct {
	proto      string
	addr       string
	configFile *auth.ConfigFile
	in         io.ReadCloser
	out        io.Writer
	err        io.Writer
	isTerminal bool
	terminalFd uintptr
}
