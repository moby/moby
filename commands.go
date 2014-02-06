package docker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dotcloud/docker/api"
	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/auth"
	"github.com/dotcloud/docker/engine"
	flag "github.com/dotcloud/docker/pkg/mflag"
	"github.com/dotcloud/docker/pkg/sysinfo"
	"github.com/dotcloud/docker/pkg/term"
	"github.com/dotcloud/docker/registry"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"text/template"
	"time"
)

var (
	GITCOMMIT string
	VERSION   string
)

var (
	ErrConnectionRefused = errors.New("Can't connect to docker daemon. Is 'docker -d' running on this host?")
)

func (cli *DockerCli) getMethod(name string) (func(...string) error, bool) {
	methodName := "Cmd" + strings.ToUpper(name[:1]) + strings.ToLower(name[1:])
	method := reflect.ValueOf(cli).MethodByName(methodName)
	if !method.IsValid() {
		return nil, false
	}
	return method.Interface().(func(...string) error), true
}

func ParseCommands(proto, addr string, args ...string) error {
	cli := NewDockerCli(os.Stdin, os.Stdout, os.Stderr, proto, addr)

	if len(args) > 0 {
		method, exists := cli.getMethod(args[0])
		if !exists {
			fmt.Println("Error: Command not found:", args[0])
			return cli.CmdHelp(args[1:]...)
		}
		return method(args[1:]...)
	}
	return cli.CmdHelp(args...)
}

func (cli *DockerCli) CmdHelp(args ...string) error {
	if len(args) > 0 {
		method, exists := cli.getMethod(args[0])
		if !exists {
			fmt.Fprintf(cli.err, "Error: Command not found: %s\n", args[0])
		} else {
			method("--help")
			return nil
		}
	}
	help := fmt.Sprintf("Usage: docker [OPTIONS] COMMAND [arg...]\n -H=[unix://%s]: tcp://host:port to bind/connect to or unix://path/to/socket to use\n\nA self-sufficient runtime for linux containers.\n\nCommands:\n", api.DEFAULTUNIXSOCKET)
	for _, command := range [][]string{
		{"attach", "Attach to a running container"},
		{"build", "Build a container from a Dockerfile"},
		{"commit", "Create a new image from a container's changes"},
		{"cp", "Copy files/folders from the containers filesystem to the host path"},
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
		{"load", "Load an image from a tar archive"},
		{"login", "Register or Login to the docker registry server"},
		{"logs", "Fetch the logs of a container"},
		{"port", "Lookup the public-facing port which is NAT-ed to PRIVATE_PORT"},
		{"ps", "List containers"},
		{"pull", "Pull an image or a repository from the docker registry server"},
		{"push", "Push an image or a repository to the docker registry server"},
		{"restart", "Restart a running container"},
		{"rm", "Remove one or more containers"},
		{"rmi", "Remove one or more images"},
		{"run", "Run a command in a new container"},
		{"save", "Save an image to a tar archive"},
		{"search", "Search for an image in the docker index"},
		{"start", "Start a stopped container"},
		{"stop", "Stop a running container"},
		{"tag", "Tag an image into a repository"},
		{"top", "Lookup the running processes of a container"},
		{"version", "Show the docker version information"},
		{"wait", "Block until a container stops, then print its exit code"},
	} {
		help += fmt.Sprintf("    %-10.10s%s\n", command[0], command[1])
	}
	fmt.Fprintf(cli.err, "%s\n", help)
	return nil
}

func (cli *DockerCli) CmdInsert(args ...string) error {
	cmd := cli.Subcmd("insert", "IMAGE URL PATH", "Insert a file from URL in the IMAGE at PATH")
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

	return cli.stream("POST", "/images/"+cmd.Arg(0)+"/insert?"+v.Encode(), nil, cli.out, nil)
}

// mkBuildContext returns an archive of an empty context with the contents
// of `dockerfile` at the path ./Dockerfile
func MkBuildContext(dockerfile string, files [][2]string) (archive.Archive, error) {
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
	cmd := cli.Subcmd("build", "[OPTIONS] PATH | URL | -", "Build a new container image from the source code at PATH")
	tag := cmd.String([]string{"t", "-tag"}, "", "Repository name (and optionally a tag) to be applied to the resulting image in case of success")
	suppressOutput := cmd.Bool([]string{"q", "-quiet"}, false, "Suppress verbose build output")
	noCache := cmd.Bool([]string{"#no-cache", "-no-cache"}, false, "Do not use cache when building the image")
	rm := cmd.Bool([]string{"#rm", "-rm"}, false, "Remove intermediate containers after a successful build")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	var (
		context  archive.Archive
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
		context, err = MkBuildContext(string(dockerfile), nil)
	} else if utils.IsURL(cmd.Arg(0)) || utils.IsGIT(cmd.Arg(0)) {
		isRemote = true
	} else {
		if _, err := os.Stat(cmd.Arg(0)); err != nil {
			return err
		}
		filename := path.Join(cmd.Arg(0), "Dockerfile")
		if _, err = os.Stat(filename); os.IsNotExist(err) {
			return fmt.Errorf("no Dockerfile found in %s", cmd.Arg(0))
		}
		context, err = archive.Tar(cmd.Arg(0), archive.Uncompressed)
	}
	var body io.Reader
	// Setup an upload progress bar
	// FIXME: ProgressReader shouldn't be this annoying to use
	if context != nil {
		sf := utils.NewStreamFormatter(false)
		body = utils.ProgressReader(ioutil.NopCloser(context), 0, cli.err, sf, true, "", "Uploading context")
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
	if *noCache {
		v.Set("nocache", "1")
	}
	if *rm {
		v.Set("rm", "1")
	}

	cli.LoadConfigFile()

	headers := http.Header(make(map[string][]string))
	buf, err := json.Marshal(cli.configFile)
	if err != nil {
		return err
	}
	headers.Add("X-Registry-Config", base64.URLEncoding.EncodeToString(buf))

	if context != nil {
		headers.Set("Content-Type", "application/tar")
	}
	err = cli.stream("POST", fmt.Sprintf("/build?%s", v.Encode()), body, cli.out, headers)
	if jerr, ok := err.(*utils.JSONError); ok {
		// If no error code is set, default to 1
		if jerr.Code == 0 {
			jerr.Code = 1
		}
		return &utils.StatusError{Status: jerr.Message, StatusCode: jerr.Code}
	}
	return err
}

// 'docker login': login / register a user to registry service.
func (cli *DockerCli) CmdLogin(args ...string) error {
	cmd := cli.Subcmd("login", "[OPTIONS] [SERVER]", "Register or Login to a docker registry server, if no server is specified \""+auth.IndexServerAddress()+"\" is the default.")

	var username, password, email string

	cmd.StringVar(&username, []string{"u", "-username"}, "", "username")
	cmd.StringVar(&password, []string{"p", "-password"}, "", "password")
	cmd.StringVar(&email, []string{"e", "-email"}, "", "email")
	err := cmd.Parse(args)
	if err != nil {
		return nil
	}
	serverAddress := auth.IndexServerAddress()
	if len(cmd.Args()) > 0 {
		serverAddress = cmd.Arg(0)
	}

	promptDefault := func(prompt string, configDefault string) {
		if configDefault == "" {
			fmt.Fprintf(cli.out, "%s: ", prompt)
		} else {
			fmt.Fprintf(cli.out, "%s (%s): ", prompt, configDefault)
		}
	}

	readInput := func(in io.Reader, out io.Writer) string {
		reader := bufio.NewReader(in)
		line, _, err := reader.ReadLine()
		if err != nil {
			fmt.Fprintln(out, err.Error())
			os.Exit(1)
		}
		return string(line)
	}

	cli.LoadConfigFile()
	authconfig, ok := cli.configFile.Configs[serverAddress]
	if !ok {
		authconfig = auth.AuthConfig{}
	}

	if username == "" {
		promptDefault("Username", authconfig.Username)
		username = readInput(cli.in, cli.out)
		if username == "" {
			username = authconfig.Username
		}
	}
	if username != authconfig.Username {
		if password == "" {
			oldState, _ := term.SaveState(cli.terminalFd)
			fmt.Fprintf(cli.out, "Password: ")
			term.DisableEcho(cli.terminalFd, oldState)

			password = readInput(cli.in, cli.out)
			fmt.Fprint(cli.out, "\n")

			term.RestoreTerminal(cli.terminalFd, oldState)
			if password == "" {
				return fmt.Errorf("Error : Password Required")
			}
		}

		if email == "" {
			promptDefault("Email", authconfig.Email)
			email = readInput(cli.in, cli.out)
			if email == "" {
				email = authconfig.Email
			}
		}
	} else {
		password = authconfig.Password
		email = authconfig.Email
	}
	authconfig.Username = username
	authconfig.Password = password
	authconfig.Email = email
	authconfig.ServerAddress = serverAddress
	cli.configFile.Configs[serverAddress] = authconfig

	stream, statusCode, err := cli.call("POST", "/auth", cli.configFile.Configs[serverAddress], false)
	if statusCode == 401 {
		delete(cli.configFile.Configs, serverAddress)
		auth.SaveConfig(cli.configFile)
		return err
	}
	if err != nil {
		return err
	}
	var out2 engine.Env
	err = out2.Decode(stream)
	if err != nil {
		cli.configFile, _ = auth.LoadConfig(os.Getenv("HOME"))
		return err
	}
	auth.SaveConfig(cli.configFile)
	if out2.Get("Status") != "" {
		fmt.Fprintf(cli.out, "%s\n", out2.Get("Status"))
	}
	return nil
}

// 'docker wait': block until a container stops
func (cli *DockerCli) CmdWait(args ...string) error {
	cmd := cli.Subcmd("wait", "CONTAINER [CONTAINER...]", "Block until a container stops, then print its exit code.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	var encounteredError error
	for _, name := range cmd.Args() {
		status, err := waitForExit(cli, name)
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to wait one or more containers")
		} else {
			fmt.Fprintf(cli.out, "%d\n", status)
		}
	}
	return encounteredError
}

// 'docker version': show version information
func (cli *DockerCli) CmdVersion(args ...string) error {
	cmd := cli.Subcmd("version", "", "Show the docker version information.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() > 0 {
		cmd.Usage()
		return nil
	}
	if VERSION != "" {
		fmt.Fprintf(cli.out, "Client version: %s\n", VERSION)
	}
	fmt.Fprintf(cli.out, "Go version (client): %s\n", runtime.Version())
	if GITCOMMIT != "" {
		fmt.Fprintf(cli.out, "Git commit (client): %s\n", GITCOMMIT)
	}

	body, _, err := readBody(cli.call("GET", "/version", nil, false))
	if err != nil {
		return err
	}

	out := engine.NewOutput()
	remoteVersion, err := out.AddEnv()
	if err != nil {
		utils.Errorf("Error reading remote version: %s\n", err)
		return err
	}
	if _, err := out.Write(body); err != nil {
		utils.Errorf("Error reading remote version: %s\n", err)
		return err
	}
	out.Close()
	fmt.Fprintf(cli.out, "Server version: %s\n", remoteVersion.Get("Version"))
	fmt.Fprintf(cli.out, "Git commit (server): %s\n", remoteVersion.Get("GitCommit"))
	fmt.Fprintf(cli.out, "Go version (server): %s\n", remoteVersion.Get("GoVersion"))
	release := utils.GetReleaseVersion()
	if release != "" {
		fmt.Fprintf(cli.out, "Last stable version: %s", release)
		if (VERSION != "" || remoteVersion.Exists("Version")) && (strings.Trim(VERSION, "-dev") != release || strings.Trim(remoteVersion.Get("Version"), "-dev") != release) {
			fmt.Fprintf(cli.out, ", please update docker")
		}
		fmt.Fprintf(cli.out, "\n")
	}
	return nil
}

// 'docker info': display system-wide information.
func (cli *DockerCli) CmdInfo(args ...string) error {
	cmd := cli.Subcmd("info", "", "Display system-wide information")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() > 0 {
		cmd.Usage()
		return nil
	}

	body, _, err := readBody(cli.call("GET", "/info", nil, false))
	if err != nil {
		return err
	}

	out := engine.NewOutput()
	remoteInfo, err := out.AddEnv()
	if err != nil {
		return err
	}

	if _, err := out.Write(body); err != nil {
		utils.Errorf("Error reading remote info: %s\n", err)
		return err
	}
	out.Close()

	fmt.Fprintf(cli.out, "Containers: %d\n", remoteInfo.GetInt("Containers"))
	fmt.Fprintf(cli.out, "Images: %d\n", remoteInfo.GetInt("Images"))
	fmt.Fprintf(cli.out, "Driver: %s\n", remoteInfo.Get("Driver"))
	var driverStatus [][2]string
	if err := remoteInfo.GetJson("DriverStatus", &driverStatus); err != nil {
		return err
	}
	for _, pair := range driverStatus {
		fmt.Fprintf(cli.out, " %s: %s\n", pair[0], pair[1])
	}
	if remoteInfo.GetBool("Debug") || os.Getenv("DEBUG") != "" {
		fmt.Fprintf(cli.out, "Debug mode (server): %v\n", remoteInfo.GetBool("Debug"))
		fmt.Fprintf(cli.out, "Debug mode (client): %v\n", os.Getenv("DEBUG") != "")
		fmt.Fprintf(cli.out, "Fds: %d\n", remoteInfo.GetInt("NFd"))
		fmt.Fprintf(cli.out, "Goroutines: %d\n", remoteInfo.GetInt("NGoroutines"))
		fmt.Fprintf(cli.out, "Execution Driver: %s\n", remoteInfo.Get("ExecutionDriver"))
		fmt.Fprintf(cli.out, "EventsListeners: %d\n", remoteInfo.GetInt("NEventsListener"))
		fmt.Fprintf(cli.out, "Kernel Version: %s\n", remoteInfo.Get("KernelVersion"))

		if initSha1 := remoteInfo.Get("InitSha1"); initSha1 != "" {
			fmt.Fprintf(cli.out, "Init SHA1: %s\n", initSha1)
		}
		if initPath := remoteInfo.Get("InitPath"); initPath != "" {
			fmt.Fprintf(cli.out, "Init Path: %s\n", initPath)
		}
	}

	if len(remoteInfo.GetList("IndexServerAddress")) != 0 {
		cli.LoadConfigFile()
		u := cli.configFile.Configs[remoteInfo.Get("IndexServerAddress")].Username
		if len(u) > 0 {
			fmt.Fprintf(cli.out, "Username: %v\n", u)
			fmt.Fprintf(cli.out, "Registry: %v\n", remoteInfo.GetList("IndexServerAddress"))
		}
	}
	if !remoteInfo.GetBool("MemoryLimit") {
		fmt.Fprintf(cli.err, "WARNING: No memory limit support\n")
	}
	if !remoteInfo.GetBool("SwapLimit") {
		fmt.Fprintf(cli.err, "WARNING: No swap limit support\n")
	}
	if !remoteInfo.GetBool("IPv4Forwarding") {
		fmt.Fprintf(cli.err, "WARNING: IPv4 forwarding is disabled.\n")
	}
	return nil
}

func (cli *DockerCli) CmdStop(args ...string) error {
	cmd := cli.Subcmd("stop", "[OPTIONS] CONTAINER [CONTAINER...]", "Stop a running container (Send SIGTERM, and then SIGKILL after grace period)")
	nSeconds := cmd.Int([]string{"t", "-time"}, 10, "Number of seconds to wait for the container to stop before killing it.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	v := url.Values{}
	v.Set("t", strconv.Itoa(*nSeconds))

	var encounteredError error
	for _, name := range cmd.Args() {
		_, _, err := readBody(cli.call("POST", "/containers/"+name+"/stop?"+v.Encode(), nil, false))
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to stop one or more containers")
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return encounteredError
}

func (cli *DockerCli) CmdRestart(args ...string) error {
	cmd := cli.Subcmd("restart", "[OPTIONS] CONTAINER [CONTAINER...]", "Restart a running container")
	nSeconds := cmd.Int([]string{"t", "-time"}, 10, "Number of seconds to try to stop for before killing the container. Once killed it will then be restarted. Default=10")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	v := url.Values{}
	v.Set("t", strconv.Itoa(*nSeconds))

	var encounteredError error
	for _, name := range cmd.Args() {
		_, _, err := readBody(cli.call("POST", "/containers/"+name+"/restart?"+v.Encode(), nil, false))
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to  restart one or more containers")
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return encounteredError
}

func (cli *DockerCli) forwardAllSignals(cid string) chan os.Signal {
	sigc := make(chan os.Signal, 1)
	utils.CatchAll(sigc)
	go func() {
		for s := range sigc {
			if s == syscall.SIGCHLD {
				continue
			}
			if _, _, err := readBody(cli.call("POST", fmt.Sprintf("/containers/%s/kill?signal=%d", cid, s), nil, false)); err != nil {
				utils.Debugf("Error sending signal: %s", err)
			}
		}
	}()
	return sigc
}

func (cli *DockerCli) CmdStart(args ...string) error {
	cmd := cli.Subcmd("start", "CONTAINER [CONTAINER...]", "Restart a stopped container")
	attach := cmd.Bool([]string{"a", "-attach"}, false, "Attach container's stdout/stderr and forward all signals to the process")
	openStdin := cmd.Bool([]string{"i", "-interactive"}, false, "Attach container's stdin")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	var cErr chan error
	var tty bool
	if *attach || *openStdin {
		if cmd.NArg() > 1 {
			return fmt.Errorf("Impossible to start and attach multiple containers at once.")
		}

		body, _, err := readBody(cli.call("GET", "/containers/"+cmd.Arg(0)+"/json", nil, false))
		if err != nil {
			return err
		}

		container := &Container{}
		err = json.Unmarshal(body, container)
		if err != nil {
			return err
		}

		tty = container.Config.Tty

		if !container.Config.Tty {
			sigc := cli.forwardAllSignals(cmd.Arg(0))
			defer utils.StopCatch(sigc)
		}

		var in io.ReadCloser

		v := url.Values{}
		v.Set("stream", "1")
		if *openStdin && container.Config.OpenStdin {
			v.Set("stdin", "1")
			in = cli.in
		}
		v.Set("stdout", "1")
		v.Set("stderr", "1")

		cErr = utils.Go(func() error {
			return cli.hijack("POST", "/containers/"+cmd.Arg(0)+"/attach?"+v.Encode(), container.Config.Tty, in, cli.out, cli.err, nil)
		})
	}

	var encounteredError error
	for _, name := range cmd.Args() {
		_, _, err := readBody(cli.call("POST", "/containers/"+name+"/start", nil, false))
		if err != nil {
			if !*attach || !*openStdin {
				fmt.Fprintf(cli.err, "%s\n", err)
				encounteredError = fmt.Errorf("Error: failed to start one or more containers")
			}
		} else {
			if !*attach || !*openStdin {
				fmt.Fprintf(cli.out, "%s\n", name)
			}
		}
	}
	if encounteredError != nil {
		if *openStdin || *attach {
			cli.in.Close()
			<-cErr
		}
		return encounteredError
	}

	if *openStdin || *attach {
		if tty && cli.isTerminal {
			if err := cli.monitorTtySize(cmd.Arg(0)); err != nil {
				utils.Errorf("Error monitoring TTY size: %s\n", err)
			}
		}
		return <-cErr
	}
	return nil
}

func (cli *DockerCli) CmdInspect(args ...string) error {
	cmd := cli.Subcmd("inspect", "CONTAINER|IMAGE [CONTAINER|IMAGE...]", "Return low-level information on a container/image")
	tmplStr := cmd.String([]string{"f", "#format", "-format"}, "", "Format the output using the given go template.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	var tmpl *template.Template
	if *tmplStr != "" {
		var err error
		if tmpl, err = template.New("").Parse(*tmplStr); err != nil {
			fmt.Fprintf(cli.err, "Template parsing error: %v\n", err)
			return &utils.StatusError{StatusCode: 64,
				Status: "Template parsing error: " + err.Error()}
		}
	}

	indented := new(bytes.Buffer)
	indented.WriteByte('[')
	status := 0

	for _, name := range cmd.Args() {
		obj, _, err := readBody(cli.call("GET", "/containers/"+name+"/json", nil, false))
		if err != nil {
			obj, _, err = readBody(cli.call("GET", "/images/"+name+"/json", nil, false))
			if err != nil {
				if strings.Contains(err.Error(), "No such") {
					fmt.Fprintf(cli.err, "Error: No such image or container: %s\n", name)
				} else {
					fmt.Fprintf(cli.err, "%s", err)
				}
				status = 1
				continue
			}
		}

		if tmpl == nil {
			if err = json.Indent(indented, obj, "", "    "); err != nil {
				fmt.Fprintf(cli.err, "%s\n", err)
				status = 1
				continue
			}
		} else {
			// Has template, will render
			var value interface{}
			if err := json.Unmarshal(obj, &value); err != nil {
				fmt.Fprintf(cli.err, "%s\n", err)
				status = 1
				continue
			}
			if err := tmpl.Execute(cli.out, value); err != nil {
				return err
			}
			cli.out.Write([]byte{'\n'})
		}
		indented.WriteString(",")
	}

	if indented.Len() > 1 {
		// Remove trailing ','
		indented.Truncate(indented.Len() - 1)
	}
	indented.WriteByte(']')

	if tmpl == nil {
		if _, err := io.Copy(cli.out, indented); err != nil {
			return err
		}
	}

	if status != 0 {
		return &utils.StatusError{StatusCode: status}
	}
	return nil
}

func (cli *DockerCli) CmdTop(args ...string) error {
	cmd := cli.Subcmd("top", "CONTAINER [ps OPTIONS]", "Lookup the running processes of a container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() == 0 {
		cmd.Usage()
		return nil
	}
	val := url.Values{}
	if cmd.NArg() > 1 {
		val.Set("ps_args", strings.Join(cmd.Args()[1:], " "))
	}

	stream, _, err := cli.call("GET", "/containers/"+cmd.Arg(0)+"/top?"+val.Encode(), nil, false)
	if err != nil {
		return err
	}
	var procs engine.Env
	if err := procs.Decode(stream); err != nil {
		return err
	}
	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	fmt.Fprintln(w, strings.Join(procs.GetList("Titles"), "\t"))
	processes := [][]string{}
	if err := procs.GetJson("Processes", &processes); err != nil {
		return err
	}
	for _, proc := range processes {
		fmt.Fprintln(w, strings.Join(proc, "\t"))
	}
	w.Flush()
	return nil
}

func (cli *DockerCli) CmdPort(args ...string) error {
	cmd := cli.Subcmd("port", "CONTAINER PRIVATE_PORT", "Lookup the public-facing port which is NAT-ed to PRIVATE_PORT")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 2 {
		cmd.Usage()
		return nil
	}

	port := cmd.Arg(1)
	proto := "tcp"
	parts := strings.SplitN(port, "/", 2)
	if len(parts) == 2 && len(parts[1]) != 0 {
		port = parts[0]
		proto = parts[1]
	}
	body, _, err := readBody(cli.call("GET", "/containers/"+cmd.Arg(0)+"/json", nil, false))
	if err != nil {
		return err
	}
	var out Container
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}

	if frontends, exists := out.NetworkSettings.Ports[Port(port+"/"+proto)]; exists && frontends != nil {
		for _, frontend := range frontends {
			fmt.Fprintf(cli.out, "%s:%s\n", frontend.HostIp, frontend.HostPort)
		}
	} else {
		return fmt.Errorf("Error: No public port '%s' published for %s", cmd.Arg(1), cmd.Arg(0))
	}
	return nil
}

// 'docker rmi IMAGE' removes all images with the name IMAGE
func (cli *DockerCli) CmdRmi(args ...string) error {
	cmd := cli.Subcmd("rmi", "IMAGE [IMAGE...]", "Remove one or more images")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	var encounteredError error
	for _, name := range cmd.Args() {
		body, _, err := readBody(cli.call("DELETE", "/images/"+name, nil, false))
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to remove one or more images")
		} else {
			outs := engine.NewTable("Created", 0)
			if _, err := outs.ReadListFrom(body); err != nil {
				fmt.Fprintf(cli.err, "%s\n", err)
				encounteredError = fmt.Errorf("Error: failed to remove one or more images")
				continue
			}
			for _, out := range outs.Data {
				if out.Get("Deleted") != "" {
					fmt.Fprintf(cli.out, "Deleted: %s\n", out.Get("Deleted"))
				} else {
					fmt.Fprintf(cli.out, "Untagged: %s\n", out.Get("Untagged"))
				}
			}
		}
	}
	return encounteredError
}

func (cli *DockerCli) CmdHistory(args ...string) error {
	cmd := cli.Subcmd("history", "[OPTIONS] IMAGE", "Show the history of an image")
	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "only show numeric IDs")
	noTrunc := cmd.Bool([]string{"#notrunc", "-no-trunc"}, false, "Don't truncate output")

	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	body, _, err := readBody(cli.call("GET", "/images/"+cmd.Arg(0)+"/history", nil, false))
	if err != nil {
		return err
	}

	outs := engine.NewTable("Created", 0)
	if _, err := outs.ReadListFrom(body); err != nil {
		return err
	}

	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	if !*quiet {
		fmt.Fprintln(w, "IMAGE\tCREATED\tCREATED BY\tSIZE")
	}

	for _, out := range outs.Data {
		outID := out.Get("Id")
		if !*quiet {
			if *noTrunc {
				fmt.Fprintf(w, "%s\t", outID)
			} else {
				fmt.Fprintf(w, "%s\t", utils.TruncateID(outID))
			}

			fmt.Fprintf(w, "%s ago\t", utils.HumanDuration(time.Now().UTC().Sub(time.Unix(out.GetInt64("Created"), 0))))

			if *noTrunc {
				fmt.Fprintf(w, "%s\t", out.Get("CreatedBy"))
			} else {
				fmt.Fprintf(w, "%s\t", utils.Trunc(out.Get("CreatedBy"), 45))
			}
			fmt.Fprintf(w, "%s\n", utils.HumanSize(out.GetInt64("Size")))
		} else {
			if *noTrunc {
				fmt.Fprintln(w, outID)
			} else {
				fmt.Fprintln(w, utils.TruncateID(outID))
			}
		}
	}
	w.Flush()
	return nil
}

func (cli *DockerCli) CmdRm(args ...string) error {
	cmd := cli.Subcmd("rm", "[OPTIONS] CONTAINER [CONTAINER...]", "Remove one or more containers")
	v := cmd.Bool([]string{"v", "-volumes"}, false, "Remove the volumes associated to the container")
	link := cmd.Bool([]string{"l", "#link", "-link"}, false, "Remove the specified link and not the underlying container")

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
	if *link {
		val.Set("link", "1")
	}

	var encounteredError error
	for _, name := range cmd.Args() {
		_, _, err := readBody(cli.call("DELETE", "/containers/"+name+"?"+val.Encode(), nil, false))
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to remove one or more containers")
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return encounteredError
}

// 'docker kill NAME' kills a running container
func (cli *DockerCli) CmdKill(args ...string) error {
	cmd := cli.Subcmd("kill", "[OPTIONS] CONTAINER [CONTAINER...]", "Kill a running container (send SIGKILL, or specified signal)")
	signal := cmd.String([]string{"s", "-signal"}, "KILL", "Signal to send to the container")

	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	var encounteredError error
	for _, name := range cmd.Args() {
		if _, _, err := readBody(cli.call("POST", fmt.Sprintf("/containers/%s/kill?signal=%s", name, *signal), nil, false)); err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to kill one or more containers")
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return encounteredError
}

func (cli *DockerCli) CmdImport(args ...string) error {
	cmd := cli.Subcmd("import", "URL|- [REPOSITORY[:TAG]]", "Create an empty filesystem image and import the contents of the tarball (.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz) into it, then optionally tag it.")

	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	var src, repository, tag string

	if cmd.NArg() == 3 {
		fmt.Fprintf(cli.err, "[DEPRECATED] The format 'URL|- [REPOSITORY [TAG]]' as been deprecated. Please use URL|- [REPOSITORY[:TAG]]\n")
		src, repository, tag = cmd.Arg(0), cmd.Arg(1), cmd.Arg(2)
	} else {
		src = cmd.Arg(0)
		repository, tag = utils.ParseRepositoryTag(cmd.Arg(1))
	}
	v := url.Values{}
	v.Set("repo", repository)
	v.Set("tag", tag)
	v.Set("fromSrc", src)

	var in io.Reader

	if src == "-" {
		in = cli.in
	}

	return cli.stream("POST", "/images/create?"+v.Encode(), in, cli.out, nil)
}

func (cli *DockerCli) CmdPush(args ...string) error {
	cmd := cli.Subcmd("push", "NAME", "Push an image or a repository to the registry")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	name := cmd.Arg(0)

	if name == "" {
		cmd.Usage()
		return nil
	}

	cli.LoadConfigFile()

	// Resolve the Repository name from fqn to endpoint + name
	endpoint, _, err := registry.ResolveRepositoryName(name)
	if err != nil {
		return err
	}
	// Resolve the Auth config relevant for this server
	authConfig := cli.configFile.ResolveAuthConfig(endpoint)
	// If we're not using a custom registry, we know the restrictions
	// applied to repository names and can warn the user in advance.
	// Custom repositories can have different rules, and we must also
	// allow pushing by image ID.
	if len(strings.SplitN(name, "/", 2)) == 1 {
		username := cli.configFile.Configs[auth.IndexServerAddress()].Username
		if username == "" {
			username = "<user>"
		}
		return fmt.Errorf("Impossible to push a \"root\" repository. Please rename your repository in <user>/<repo> (ex: %s/%s)", username, name)
	}

	v := url.Values{}
	push := func(authConfig auth.AuthConfig) error {
		buf, err := json.Marshal(authConfig)
		if err != nil {
			return err
		}
		registryAuthHeader := []string{
			base64.URLEncoding.EncodeToString(buf),
		}

		return cli.stream("POST", "/images/"+name+"/push?"+v.Encode(), nil, cli.out, map[string][]string{
			"X-Registry-Auth": registryAuthHeader,
		})
	}

	if err := push(authConfig); err != nil {
		if strings.Contains(err.Error(), "Status 401") {
			fmt.Fprintln(cli.out, "\nPlease login prior to push:")
			if err := cli.CmdLogin(endpoint); err != nil {
				return err
			}
			authConfig := cli.configFile.ResolveAuthConfig(endpoint)
			return push(authConfig)
		}
		return err
	}
	return nil
}

func (cli *DockerCli) CmdPull(args ...string) error {
	cmd := cli.Subcmd("pull", "NAME", "Pull an image or a repository from the registry")
	tag := cmd.String([]string{"t", "-tag"}, "", "Download tagged image in repository")
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

	// Resolve the Repository name from fqn to endpoint + name
	endpoint, _, err := registry.ResolveRepositoryName(remote)
	if err != nil {
		return err
	}

	cli.LoadConfigFile()

	// Resolve the Auth config relevant for this server
	authConfig := cli.configFile.ResolveAuthConfig(endpoint)
	v := url.Values{}
	v.Set("fromImage", remote)
	v.Set("tag", *tag)

	pull := func(authConfig auth.AuthConfig) error {
		buf, err := json.Marshal(authConfig)
		if err != nil {
			return err
		}
		registryAuthHeader := []string{
			base64.URLEncoding.EncodeToString(buf),
		}

		return cli.stream("POST", "/images/create?"+v.Encode(), nil, cli.out, map[string][]string{
			"X-Registry-Auth": registryAuthHeader,
		})
	}

	if err := pull(authConfig); err != nil {
		if strings.Contains(err.Error(), "Status 401") {
			fmt.Fprintln(cli.out, "\nPlease login prior to pull:")
			if err := cli.CmdLogin(endpoint); err != nil {
				return err
			}
			authConfig := cli.configFile.ResolveAuthConfig(endpoint)
			return pull(authConfig)
		}
		return err
	}

	return nil
}

func (cli *DockerCli) CmdImages(args ...string) error {
	cmd := cli.Subcmd("images", "[OPTIONS] [NAME]", "List images")
	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "only show numeric IDs")
	all := cmd.Bool([]string{"a", "-all"}, false, "show all images (by default filter out the intermediate images used to build)")
	noTrunc := cmd.Bool([]string{"#notrunc", "-no-trunc"}, false, "Don't truncate output")
	flViz := cmd.Bool([]string{"v", "#viz", "-viz"}, false, "output graph in graphviz format")
	flTree := cmd.Bool([]string{"t", "#tree", "-tree"}, false, "output graph in tree format")

	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() > 1 {
		cmd.Usage()
		return nil
	}

	filter := cmd.Arg(0)

	if *flViz || *flTree {
		body, _, err := readBody(cli.call("GET", "/images/json?all=1", nil, false))
		if err != nil {
			return err
		}

		outs := engine.NewTable("Created", 0)
		if _, err := outs.ReadListFrom(body); err != nil {
			return err
		}

		var (
			printNode  func(cli *DockerCli, noTrunc bool, image *engine.Env, prefix string)
			startImage *engine.Env

			roots    = engine.NewTable("Created", outs.Len())
			byParent = make(map[string]*engine.Table)
		)

		for _, image := range outs.Data {
			if image.Get("ParentId") == "" {
				roots.Add(image)
			} else {
				if children, exists := byParent[image.Get("ParentId")]; exists {
					children.Add(image)
				} else {
					byParent[image.Get("ParentId")] = engine.NewTable("Created", 1)
					byParent[image.Get("ParentId")].Add(image)
				}
			}

			if filter != "" {
				if filter == image.Get("Id") || filter == utils.TruncateID(image.Get("Id")) {
					startImage = image
				}

				for _, repotag := range image.GetList("RepoTags") {
					if repotag == filter {
						startImage = image
					}
				}
			}
		}

		if *flViz {
			fmt.Fprintf(cli.out, "digraph docker {\n")
			printNode = (*DockerCli).printVizNode
		} else {
			printNode = (*DockerCli).printTreeNode
		}

		if startImage != nil {
			root := engine.NewTable("Created", 1)
			root.Add(startImage)
			cli.WalkTree(*noTrunc, root, byParent, "", printNode)
		} else if filter == "" {
			cli.WalkTree(*noTrunc, roots, byParent, "", printNode)
		}
		if *flViz {
			fmt.Fprintf(cli.out, " base [style=invisible]\n}\n")
		}
	} else {
		v := url.Values{}
		if cmd.NArg() == 1 {
			v.Set("filter", filter)
		}
		if *all {
			v.Set("all", "1")
		}

		body, _, err := readBody(cli.call("GET", "/images/json?"+v.Encode(), nil, false))

		if err != nil {
			return err
		}

		outs := engine.NewTable("Created", 0)
		if _, err := outs.ReadListFrom(body); err != nil {
			return err
		}

		w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
		if !*quiet {
			fmt.Fprintln(w, "REPOSITORY\tTAG\tIMAGE ID\tCREATED\tVIRTUAL SIZE")
		}

		for _, out := range outs.Data {
			for _, repotag := range out.GetList("RepoTags") {

				repo, tag := utils.ParseRepositoryTag(repotag)
				outID := out.Get("Id")
				if !*noTrunc {
					outID = utils.TruncateID(outID)
				}

				if !*quiet {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s ago\t%s\n", repo, tag, outID, utils.HumanDuration(time.Now().UTC().Sub(time.Unix(out.GetInt64("Created"), 0))), utils.HumanSize(out.GetInt64("VirtualSize")))
				} else {
					fmt.Fprintln(w, outID)
				}
			}
		}

		if !*quiet {
			w.Flush()
		}
	}
	return nil
}

func (cli *DockerCli) WalkTree(noTrunc bool, images *engine.Table, byParent map[string]*engine.Table, prefix string, printNode func(cli *DockerCli, noTrunc bool, image *engine.Env, prefix string)) {
	length := images.Len()
	if length > 1 {
		for index, image := range images.Data {
			if index+1 == length {
				printNode(cli, noTrunc, image, prefix+"└─")
				if subimages, exists := byParent[image.Get("Id")]; exists {
					cli.WalkTree(noTrunc, subimages, byParent, prefix+"  ", printNode)
				}
			} else {
				printNode(cli, noTrunc, image, prefix+"\u251C─")
				if subimages, exists := byParent[image.Get("Id")]; exists {
					cli.WalkTree(noTrunc, subimages, byParent, prefix+"\u2502 ", printNode)
				}
			}
		}
	} else {
		for _, image := range images.Data {
			printNode(cli, noTrunc, image, prefix+"└─")
			if subimages, exists := byParent[image.Get("Id")]; exists {
				cli.WalkTree(noTrunc, subimages, byParent, prefix+"  ", printNode)
			}
		}
	}
}

func (cli *DockerCli) printVizNode(noTrunc bool, image *engine.Env, prefix string) {
	var (
		imageID  string
		parentID string
	)
	if noTrunc {
		imageID = image.Get("Id")
		parentID = image.Get("ParentId")
	} else {
		imageID = utils.TruncateID(image.Get("Id"))
		parentID = utils.TruncateID(image.Get("ParentId"))
	}
	if parentID == "" {
		fmt.Fprintf(cli.out, " base -> \"%s\" [style=invis]\n", imageID)
	} else {
		fmt.Fprintf(cli.out, " \"%s\" -> \"%s\"\n", parentID, imageID)
	}
	if image.GetList("RepoTags")[0] != "<none>:<none>" {
		fmt.Fprintf(cli.out, " \"%s\" [label=\"%s\\n%s\",shape=box,fillcolor=\"paleturquoise\",style=\"filled,rounded\"];\n",
			imageID, imageID, strings.Join(image.GetList("RepoTags"), "\\n"))
	}
}

func (cli *DockerCli) printTreeNode(noTrunc bool, image *engine.Env, prefix string) {
	var imageID string
	if noTrunc {
		imageID = image.Get("Id")
	} else {
		imageID = utils.TruncateID(image.Get("Id"))
	}

	fmt.Fprintf(cli.out, "%s%s Virtual Size: %s", prefix, imageID, utils.HumanSize(image.GetInt64("VirtualSize")))
	if image.GetList("RepoTags")[0] != "<none>:<none>" {
		fmt.Fprintf(cli.out, " Tags: %s\n", strings.Join(image.GetList("RepoTags"), ", "))
	} else {
		fmt.Fprint(cli.out, "\n")
	}
}

func displayablePorts(ports *engine.Table) string {
	result := []string{}
	for _, port := range ports.Data {
		if port.Get("IP") == "" {
			result = append(result, fmt.Sprintf("%d/%s", port.GetInt("PublicPort"), port.Get("Type")))
		} else {
			result = append(result, fmt.Sprintf("%s:%d->%d/%s", port.Get("IP"), port.GetInt("PublicPort"), port.GetInt("PrivatePort"), port.Get("Type")))
		}
	}
	sort.Strings(result)
	return strings.Join(result, ", ")
}

func (cli *DockerCli) CmdPs(args ...string) error {
	cmd := cli.Subcmd("ps", "[OPTIONS]", "List containers")
	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "Only display numeric IDs")
	size := cmd.Bool([]string{"s", "-size"}, false, "Display sizes")
	all := cmd.Bool([]string{"a", "-all"}, false, "Show all containers. Only running containers are shown by default.")
	noTrunc := cmd.Bool([]string{"#notrunc", "-no-trunc"}, false, "Don't truncate output")
	nLatest := cmd.Bool([]string{"l", "-latest"}, false, "Show only the latest created container, include non-running ones.")
	since := cmd.String([]string{"#sinceId", "-since-id"}, "", "Show only containers created since Id, include non-running ones.")
	before := cmd.String([]string{"#beforeId", "-before-id"}, "", "Show only container created before Id, include non-running ones.")
	last := cmd.Int([]string{"n"}, -1, "Show n last created containers, include non-running ones.")

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

	body, _, err := readBody(cli.call("GET", "/containers/json?"+v.Encode(), nil, false))
	if err != nil {
		return err
	}

	outs := engine.NewTable("Created", 0)
	if _, err := outs.ReadListFrom(body); err != nil {
		return err
	}
	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	if !*quiet {
		fmt.Fprint(w, "CONTAINER ID\tIMAGE\tCOMMAND\tCREATED\tSTATUS\tPORTS\tNAMES")
		if *size {
			fmt.Fprintln(w, "\tSIZE")
		} else {
			fmt.Fprint(w, "\n")
		}
	}

	for _, out := range outs.Data {
		var (
			outID    = out.Get("Id")
			outNames = out.GetList("Names")
		)

		if !*noTrunc {
			outID = utils.TruncateID(outID)
		}

		// Remove the leading / from the names
		for i := 0; i < len(outNames); i++ {
			outNames[i] = outNames[i][1:]
		}

		if !*quiet {
			var (
				outCommand = out.Get("Command")
				ports      = engine.NewTable("", 0)
			)
			if !*noTrunc {
				outCommand = utils.Trunc(outCommand, 20)
			}
			ports.ReadListFrom([]byte(out.Get("Ports")))
			fmt.Fprintf(w, "%s\t%s\t%s\t%s ago\t%s\t%s\t%s\t", outID, out.Get("Image"), outCommand, utils.HumanDuration(time.Now().UTC().Sub(time.Unix(out.GetInt64("Created"), 0))), out.Get("Status"), displayablePorts(ports), strings.Join(outNames, ","))
			if *size {
				if out.GetInt("SizeRootFs") > 0 {
					fmt.Fprintf(w, "%s (virtual %s)\n", utils.HumanSize(out.GetInt64("SizeRw")), utils.HumanSize(out.GetInt64("SizeRootFs")))
				} else {
					fmt.Fprintf(w, "%s\n", utils.HumanSize(out.GetInt64("SizeRw")))
				}
			} else {
				fmt.Fprint(w, "\n")
			}
		} else {
			fmt.Fprintln(w, outID)
		}
	}

	if !*quiet {
		w.Flush()
	}
	return nil
}

func (cli *DockerCli) CmdCommit(args ...string) error {
	cmd := cli.Subcmd("commit", "[OPTIONS] CONTAINER [REPOSITORY[:TAG]]", "Create a new image from a container's changes")
	flComment := cmd.String([]string{"m", "-message"}, "", "Commit message")
	flAuthor := cmd.String([]string{"a", "#author", "-author"}, "", "Author (eg. \"John Hannibal Smith <hannibal@a-team.com>\"")
	flConfig := cmd.String([]string{"#run", "-run"}, "", "Config automatically applied when the image is run. "+`(ex: -run='{"Cmd": ["cat", "/world"], "PortSpecs": ["22"]}')`)
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	var name, repository, tag string

	if cmd.NArg() == 3 {
		fmt.Fprintf(cli.err, "[DEPRECATED] The format 'CONTAINER [REPOSITORY [TAG]]' as been deprecated. Please use CONTAINER [REPOSITORY[:TAG]]\n")
		name, repository, tag = cmd.Arg(0), cmd.Arg(1), cmd.Arg(2)
	} else {
		name = cmd.Arg(0)
		repository, tag = utils.ParseRepositoryTag(cmd.Arg(1))
	}

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
	var (
		config *Config
		env    engine.Env
	)
	if *flConfig != "" {
		config = &Config{}
		if err := json.Unmarshal([]byte(*flConfig), config); err != nil {
			return err
		}
	}
	stream, _, err := cli.call("POST", "/commit?"+v.Encode(), config, false)
	if err != nil {
		return err
	}
	if err := env.Decode(stream); err != nil {
		return err
	}

	fmt.Fprintf(cli.out, "%s\n", env.Get("Id"))
	return nil
}

func (cli *DockerCli) CmdEvents(args ...string) error {
	cmd := cli.Subcmd("events", "[OPTIONS]", "Get real time events from the server")
	since := cmd.String([]string{"#since", "-since"}, "", "Show previously created events and then stream.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() != 0 {
		cmd.Usage()
		return nil
	}

	v := url.Values{}
	if *since != "" {
		loc := time.FixedZone(time.Now().Zone())
		format := "2006-01-02 15:04:05 -0700 MST"
		if len(*since) < len(format) {
			format = format[:len(*since)]
		}

		if t, err := time.ParseInLocation(format, *since, loc); err == nil {
			v.Set("since", strconv.FormatInt(t.Unix(), 10))
		} else {
			v.Set("since", *since)
		}
	}

	if err := cli.stream("GET", "/events?"+v.Encode(), nil, cli.out, nil); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) CmdExport(args ...string) error {
	cmd := cli.Subcmd("export", "CONTAINER", "Export the contents of a filesystem as a tar archive to STDOUT")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	if err := cli.stream("GET", "/containers/"+cmd.Arg(0)+"/export", nil, cli.out, nil); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) CmdDiff(args ...string) error {
	cmd := cli.Subcmd("diff", "CONTAINER", "Inspect changes on a container's filesystem")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	body, _, err := readBody(cli.call("GET", "/containers/"+cmd.Arg(0)+"/changes", nil, false))

	if err != nil {
		return err
	}

	outs := engine.NewTable("", 0)
	if _, err := outs.ReadListFrom(body); err != nil {
		return err
	}
	for _, change := range outs.Data {
		var kind string
		switch change.GetInt("Kind") {
		case archive.ChangeModify:
			kind = "C"
		case archive.ChangeAdd:
			kind = "A"
		case archive.ChangeDelete:
			kind = "D"
		}
		fmt.Fprintf(cli.out, "%s %s\n", kind, change.Get("Path"))
	}
	return nil
}

func (cli *DockerCli) CmdLogs(args ...string) error {
	cmd := cli.Subcmd("logs", "CONTAINER", "Fetch the logs of a container")
	follow := cmd.Bool([]string{"f", "-follow"}, false, "Follow log output")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}
	name := cmd.Arg(0)
	body, _, err := readBody(cli.call("GET", "/containers/"+name+"/json", nil, false))
	if err != nil {
		return err
	}

	container := &Container{}
	err = json.Unmarshal(body, container)
	if err != nil {
		return err
	}

	v := url.Values{}
	v.Set("logs", "1")
	v.Set("stdout", "1")
	v.Set("stderr", "1")
	if *follow && container.State.Running {
		v.Set("stream", "1")
	}

	if err := cli.hijack("POST", "/containers/"+name+"/attach?"+v.Encode(), container.Config.Tty, nil, cli.out, cli.err, nil); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) CmdAttach(args ...string) error {
	cmd := cli.Subcmd("attach", "[OPTIONS] CONTAINER", "Attach to a running container")
	noStdin := cmd.Bool([]string{"#nostdin", "-no-stdin"}, false, "Do not attach stdin")
	proxy := cmd.Bool([]string{"#sig-proxy", "-sig-proxy"}, true, "Proxify all received signal to the process (even in non-tty mode)")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}
	name := cmd.Arg(0)
	body, _, err := readBody(cli.call("GET", "/containers/"+name+"/json", nil, false))
	if err != nil {
		return err
	}

	container := &Container{}
	err = json.Unmarshal(body, container)
	if err != nil {
		return err
	}

	if !container.State.IsRunning() {
		return fmt.Errorf("Impossible to attach to a stopped container, start it first")
	}

	if container.Config.Tty && cli.isTerminal {
		if err := cli.monitorTtySize(cmd.Arg(0)); err != nil {
			utils.Debugf("Error monitoring TTY size: %s", err)
		}
	}

	var in io.ReadCloser

	v := url.Values{}
	v.Set("stream", "1")
	if !*noStdin && container.Config.OpenStdin {
		v.Set("stdin", "1")
		in = cli.in
	}
	v.Set("stdout", "1")
	v.Set("stderr", "1")

	if *proxy && !container.Config.Tty {
		sigc := cli.forwardAllSignals(cmd.Arg(0))
		defer utils.StopCatch(sigc)
	}

	if err := cli.hijack("POST", "/containers/"+cmd.Arg(0)+"/attach?"+v.Encode(), container.Config.Tty, in, cli.out, cli.err, nil); err != nil {
		return err
	}

	_, status, err := getExitCode(cli, cmd.Arg(0))
	if err != nil {
		return err
	}
	if status != 0 {
		return &utils.StatusError{StatusCode: status}
	}

	return nil
}

func (cli *DockerCli) CmdSearch(args ...string) error {
	cmd := cli.Subcmd("search", "TERM", "Search the docker index for images")
	noTrunc := cmd.Bool([]string{"#notrunc", "-no-trunc"}, false, "Don't truncate output")
	trusted := cmd.Bool([]string{"t", "#trusted", "-trusted"}, false, "Only show trusted builds")
	stars := cmd.Int([]string{"s", "#stars", "-stars"}, 0, "Only displays with at least xxx stars")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	v := url.Values{}
	v.Set("term", cmd.Arg(0))

	body, _, err := readBody(cli.call("GET", "/images/search?"+v.Encode(), nil, true))

	if err != nil {
		return err
	}
	outs := engine.NewTable("star_count", 0)
	if _, err := outs.ReadListFrom(body); err != nil {
		return err
	}
	w := tabwriter.NewWriter(cli.out, 10, 1, 3, ' ', 0)
	fmt.Fprintf(w, "NAME\tDESCRIPTION\tSTARS\tOFFICIAL\tTRUSTED\n")
	for _, out := range outs.Data {
		if (*trusted && !out.GetBool("is_trusted")) || (*stars > out.GetInt("star_count")) {
			continue
		}
		desc := strings.Replace(out.Get("description"), "\n", " ", -1)
		desc = strings.Replace(desc, "\r", " ", -1)
		if !*noTrunc && len(desc) > 45 {
			desc = utils.Trunc(desc, 42) + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t", out.Get("name"), desc, out.GetInt("star_count"))
		if out.GetBool("is_official") {
			fmt.Fprint(w, "[OK]")

		}
		fmt.Fprint(w, "\t")
		if out.GetBool("is_trusted") {
			fmt.Fprint(w, "[OK]")
		}
		fmt.Fprint(w, "\n")
	}
	w.Flush()
	return nil
}

// Ports type - Used to parse multiple -p flags
type ports []int

func (cli *DockerCli) CmdTag(args ...string) error {
	cmd := cli.Subcmd("tag", "[OPTIONS] IMAGE REPOSITORY[:TAG]", "Tag an image into a repository")
	force := cmd.Bool([]string{"f", "#force", "-force"}, false, "Force")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 2 && cmd.NArg() != 3 {
		cmd.Usage()
		return nil
	}

	var repository, tag string

	if cmd.NArg() == 3 {
		fmt.Fprintf(cli.err, "[DEPRECATED] The format 'IMAGE [REPOSITORY [TAG]]' as been deprecated. Please use IMAGE [REPOSITORY[:TAG]]\n")
		repository, tag = cmd.Arg(1), cmd.Arg(2)
	} else {
		repository, tag = utils.ParseRepositoryTag(cmd.Arg(1))
	}

	v := url.Values{}
	v.Set("repo", repository)
	v.Set("tag", tag)

	if *force {
		v.Set("force", "1")
	}

	if _, _, err := readBody(cli.call("POST", "/images/"+cmd.Arg(0)+"/tag?"+v.Encode(), nil, false)); err != nil {
		return err
	}
	return nil
}

//FIXME Only used in tests
func ParseRun(args []string, sysInfo *sysinfo.SysInfo) (*Config, *HostConfig, *flag.FlagSet, error) {
	cmd := flag.NewFlagSet("run", flag.ContinueOnError)
	cmd.SetOutput(ioutil.Discard)
	cmd.Usage = nil
	return parseRun(cmd, args, sysInfo)
}

func parseRun(cmd *flag.FlagSet, args []string, sysInfo *sysinfo.SysInfo) (*Config, *HostConfig, *flag.FlagSet, error) {
	var (
		// FIXME: use utils.ListOpts for attach and volumes?
		flAttach  = NewListOpts(ValidateAttach)
		flVolumes = NewListOpts(ValidatePath)
		flLinks   = NewListOpts(ValidateLink)
		flEnv     = NewListOpts(ValidateEnv)

		flPublish     ListOpts
		flExpose      ListOpts
		flDns         ListOpts
		flVolumesFrom ListOpts
		flLxcOpts     ListOpts

		flAutoRemove      = cmd.Bool([]string{"#rm", "-rm"}, false, "Automatically remove the container when it exits (incompatible with -d)")
		flDetach          = cmd.Bool([]string{"d", "-detach"}, false, "Detached mode: Run container in the background, print new container id")
		flNetwork         = cmd.Bool([]string{"n", "-networking"}, true, "Enable networking for this container")
		flPrivileged      = cmd.Bool([]string{"#privileged", "-privileged"}, false, "Give extended privileges to this container")
		flPublishAll      = cmd.Bool([]string{"P", "-publish-all"}, false, "Publish all exposed ports to the host interfaces")
		flStdin           = cmd.Bool([]string{"i", "-interactive"}, false, "Keep stdin open even if not attached")
		flTty             = cmd.Bool([]string{"t", "-tty"}, false, "Allocate a pseudo-tty")
		flContainerIDFile = cmd.String([]string{"#cidfile", "-cidfile"}, "", "Write the container ID to the file")
		flEntrypoint      = cmd.String([]string{"#entrypoint", "-entrypoint"}, "", "Overwrite the default entrypoint of the image")
		flHostname        = cmd.String([]string{"h", "-hostname"}, "", "Container host name")
		flMemoryString    = cmd.String([]string{"m", "-memory"}, "", "Memory limit (format: <number><optional unit>, where unit = b, k, m or g)")
		flUser            = cmd.String([]string{"u", "-user"}, "", "Username or UID")
		flWorkingDir      = cmd.String([]string{"w", "-workdir"}, "", "Working directory inside the container")
		flCpuShares       = cmd.Int64([]string{"c", "-cpu-shares"}, 0, "CPU shares (relative weight)")

		// For documentation purpose
		_ = cmd.Bool([]string{"#sig-proxy", "-sig-proxy"}, true, "Proxify all received signal to the process (even in non-tty mode)")
		_ = cmd.String([]string{"#name", "-name"}, "", "Assign a name to the container")
	)

	cmd.Var(&flAttach, []string{"a", "-attach"}, "Attach to stdin, stdout or stderr.")
	cmd.Var(&flVolumes, []string{"v", "-volume"}, "Bind mount a volume (e.g. from the host: -v /host:/container, from docker: -v /container)")
	cmd.Var(&flLinks, []string{"#link", "-link"}, "Add link to another container (name:alias)")
	cmd.Var(&flEnv, []string{"e", "-env"}, "Set environment variables")

	cmd.Var(&flPublish, []string{"p", "-publish"}, fmt.Sprintf("Publish a container's port to the host (format: %s) (use 'docker port' to see the actual mapping)", PortSpecTemplateFormat))
	cmd.Var(&flExpose, []string{"#expose", "-expose"}, "Expose a port from the container without publishing it to your host")
	cmd.Var(&flDns, []string{"#dns", "-dns"}, "Set custom dns servers")
	cmd.Var(&flVolumesFrom, []string{"#volumes-from", "-volumes-from"}, "Mount volumes from the specified container(s)")
	cmd.Var(&flLxcOpts, []string{"#lxc-conf", "-lxc-conf"}, "Add custom lxc options -lxc-conf=\"lxc.cgroup.cpuset.cpus = 0,1\"")

	if err := cmd.Parse(args); err != nil {
		return nil, nil, cmd, err
	}

	// Check if the kernel supports memory limit cgroup.
	if sysInfo != nil && *flMemoryString != "" && !sysInfo.MemoryLimit {
		*flMemoryString = ""
	}

	// Validate input params
	if *flDetach && flAttach.Len() > 0 {
		return nil, nil, cmd, ErrConflictAttachDetach
	}
	if *flWorkingDir != "" && !path.IsAbs(*flWorkingDir) {
		return nil, nil, cmd, ErrInvalidWorikingDirectory
	}
	if *flDetach && *flAutoRemove {
		return nil, nil, cmd, ErrConflictDetachAutoRemove
	}

	// If neither -d or -a are set, attach to everything by default
	if flAttach.Len() == 0 && !*flDetach {
		if !*flDetach {
			flAttach.Set("stdout")
			flAttach.Set("stderr")
			if *flStdin {
				flAttach.Set("stdin")
			}
		}
	}

	var flMemory int64
	if *flMemoryString != "" {
		parsedMemory, err := utils.RAMInBytes(*flMemoryString)
		if err != nil {
			return nil, nil, cmd, err
		}
		flMemory = parsedMemory
	}

	var binds []string
	// add any bind targets to the list of container volumes
	for bind := range flVolumes.GetMap() {
		if arr := strings.Split(bind, ":"); len(arr) > 1 {
			if arr[0] == "/" {
				return nil, nil, cmd, fmt.Errorf("Invalid bind mount: source can't be '/'")
			}
			dstDir := arr[1]
			flVolumes.Set(dstDir)
			binds = append(binds, bind)
			flVolumes.Delete(bind)
		} else if bind == "/" {
			return nil, nil, cmd, fmt.Errorf("Invalid volume: path can't be '/'")
		}
	}

	var (
		parsedArgs = cmd.Args()
		runCmd     []string
		entrypoint []string
		image      string
	)
	if len(parsedArgs) >= 1 {
		image = cmd.Arg(0)
	}
	if len(parsedArgs) > 1 {
		runCmd = parsedArgs[1:]
	}
	if *flEntrypoint != "" {
		entrypoint = []string{*flEntrypoint}
	}

	lxcConf, err := parseLxcConfOpts(flLxcOpts)
	if err != nil {
		return nil, nil, cmd, err
	}

	var (
		domainname string
		hostname   = *flHostname
		parts      = strings.SplitN(hostname, ".", 2)
	)
	if len(parts) > 1 {
		hostname = parts[0]
		domainname = parts[1]
	}

	ports, portBindings, err := parsePortSpecs(flPublish.GetAll())
	if err != nil {
		return nil, nil, cmd, err
	}

	// Merge in exposed ports to the map of published ports
	for _, e := range flExpose.GetAll() {
		if strings.Contains(e, ":") {
			return nil, nil, cmd, fmt.Errorf("Invalid port format for --expose: %s", e)
		}
		p := NewPort(splitProtoPort(e))
		if _, exists := ports[p]; !exists {
			ports[p] = struct{}{}
		}
	}

	config := &Config{
		Hostname:        hostname,
		Domainname:      domainname,
		PortSpecs:       nil, // Deprecated
		ExposedPorts:    ports,
		User:            *flUser,
		Tty:             *flTty,
		NetworkDisabled: !*flNetwork,
		OpenStdin:       *flStdin,
		Memory:          flMemory,
		CpuShares:       *flCpuShares,
		AttachStdin:     flAttach.Get("stdin"),
		AttachStdout:    flAttach.Get("stdout"),
		AttachStderr:    flAttach.Get("stderr"),
		Env:             flEnv.GetAll(),
		Cmd:             runCmd,
		Dns:             flDns.GetAll(),
		Image:           image,
		Volumes:         flVolumes.GetMap(),
		VolumesFrom:     strings.Join(flVolumesFrom.GetAll(), ","),
		Entrypoint:      entrypoint,
		WorkingDir:      *flWorkingDir,
	}

	hostConfig := &HostConfig{
		Binds:           binds,
		ContainerIDFile: *flContainerIDFile,
		LxcConf:         lxcConf,
		Privileged:      *flPrivileged,
		PortBindings:    portBindings,
		Links:           flLinks.GetAll(),
		PublishAllPorts: *flPublishAll,
	}

	if sysInfo != nil && flMemory > 0 && !sysInfo.SwapLimit {
		//fmt.Fprintf(stdout, "WARNING: Your kernel does not support swap limit capabilities. Limitation discarded.\n")
		config.MemorySwap = -1
	}

	// When allocating stdin in attached mode, close stdin at client disconnect
	if config.OpenStdin && config.AttachStdin {
		config.StdinOnce = true
	}
	return config, hostConfig, cmd, nil
}

func (cli *DockerCli) CmdRun(args ...string) error {
	config, hostConfig, cmd, err := parseRun(cli.Subcmd("run", "[OPTIONS] IMAGE [COMMAND] [ARG...]", "Run a command in a new container"), args, nil)
	if err != nil {
		return err
	}
	if config.Image == "" {
		cmd.Usage()
		return nil
	}

	// Retrieve relevant client-side config
	var (
		flName        = cmd.Lookup("name")
		flRm          = cmd.Lookup("rm")
		flSigProxy    = cmd.Lookup("sig-proxy")
		autoRemove, _ = strconv.ParseBool(flRm.Value.String())
		sigProxy, _   = strconv.ParseBool(flSigProxy.Value.String())
	)

	// Disable sigProxy in case on TTY
	if config.Tty {
		sigProxy = false
	}

	var containerIDFile io.WriteCloser
	if len(hostConfig.ContainerIDFile) > 0 {
		if _, err := os.Stat(hostConfig.ContainerIDFile); err == nil {
			return fmt.Errorf("cid file found, make sure the other container isn't running or delete %s", hostConfig.ContainerIDFile)
		}
		if containerIDFile, err = os.Create(hostConfig.ContainerIDFile); err != nil {
			return fmt.Errorf("failed to create the container ID file: %s", err)
		}
		defer containerIDFile.Close()
	}

	containerValues := url.Values{}
	if name := flName.Value.String(); name != "" {
		containerValues.Set("name", name)
	}

	//create the container
	stream, statusCode, err := cli.call("POST", "/containers/create?"+containerValues.Encode(), config, false)
	//if image not found try to pull it
	if statusCode == 404 {
		_, tag := utils.ParseRepositoryTag(config.Image)
		if tag == "" {
			tag = DEFAULTTAG
		}

		fmt.Fprintf(cli.err, "Unable to find image '%s' (tag: %s) locally\n", config.Image, tag)

		v := url.Values{}
		repos, tag := utils.ParseRepositoryTag(config.Image)
		v.Set("fromImage", repos)
		v.Set("tag", tag)

		// Resolve the Repository name from fqn to endpoint + name
		endpoint, _, err := registry.ResolveRepositoryName(repos)
		if err != nil {
			return err
		}

		// Load the auth config file, to be able to pull the image
		cli.LoadConfigFile()

		// Resolve the Auth config relevant for this server
		authConfig := cli.configFile.ResolveAuthConfig(endpoint)
		buf, err := json.Marshal(authConfig)
		if err != nil {
			return err
		}

		registryAuthHeader := []string{
			base64.URLEncoding.EncodeToString(buf),
		}
		if err = cli.stream("POST", "/images/create?"+v.Encode(), nil, cli.err, map[string][]string{"X-Registry-Auth": registryAuthHeader}); err != nil {
			return err
		}
		if stream, _, err = cli.call("POST", "/containers/create?"+containerValues.Encode(), config, false); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	var runResult engine.Env
	if err := runResult.Decode(stream); err != nil {
		return err
	}

	for _, warning := range runResult.GetList("Warnings") {
		fmt.Fprintf(cli.err, "WARNING: %s\n", warning)
	}

	if len(hostConfig.ContainerIDFile) > 0 {
		if _, err = containerIDFile.Write([]byte(runResult.Get("Id"))); err != nil {
			return fmt.Errorf("failed to write the container ID to the file: %s", err)
		}
	}

	if sigProxy {
		sigc := cli.forwardAllSignals(runResult.Get("Id"))
		defer utils.StopCatch(sigc)
	}

	var (
		waitDisplayId chan struct{}
		errCh         chan error
	)

	if !config.AttachStdout && !config.AttachStderr {
		// Make this asynchrone in order to let the client write to stdin before having to read the ID
		waitDisplayId = make(chan struct{})
		go func() {
			defer close(waitDisplayId)
			fmt.Fprintf(cli.out, "%s\n", runResult.Get("Id"))
		}()
	}

	// We need to instanciate the chan because the select needs it. It can
	// be closed but can't be uninitialized.
	hijacked := make(chan io.Closer)

	// Block the return until the chan gets closed
	defer func() {
		utils.Debugf("End of CmdRun(), Waiting for hijack to finish.")
		if _, ok := <-hijacked; ok {
			utils.Errorf("Hijack did not finish (chan still open)")
		}
	}()

	if config.AttachStdin || config.AttachStdout || config.AttachStderr {
		var (
			out, stderr io.Writer
			in          io.ReadCloser
			v           = url.Values{}
		)
		v.Set("stream", "1")

		if config.AttachStdin {
			v.Set("stdin", "1")
			in = cli.in
		}
		if config.AttachStdout {
			v.Set("stdout", "1")
			out = cli.out
		}
		if config.AttachStderr {
			v.Set("stderr", "1")
			if config.Tty {
				stderr = cli.out
			} else {
				stderr = cli.err
			}
		}

		errCh = utils.Go(func() error {
			return cli.hijack("POST", "/containers/"+runResult.Get("Id")+"/attach?"+v.Encode(), config.Tty, in, out, stderr, hijacked)
		})
	} else {
		close(hijacked)
	}

	// Acknowledge the hijack before starting
	select {
	case closer := <-hijacked:
		// Make sure that hijack gets closed when returning. (result
		// in closing hijack chan and freeing server's goroutines.
		if closer != nil {
			defer closer.Close()
		}
	case err := <-errCh:
		if err != nil {
			utils.Debugf("Error hijack: %s", err)
			return err
		}
	}

	//start the container
	if _, _, err = readBody(cli.call("POST", "/containers/"+runResult.Get("Id")+"/start", hostConfig, false)); err != nil {
		return err
	}

	if (config.AttachStdin || config.AttachStdout || config.AttachStderr) && config.Tty && cli.isTerminal {
		if err := cli.monitorTtySize(runResult.Get("Id")); err != nil {
			utils.Errorf("Error monitoring TTY size: %s\n", err)
		}
	}

	if errCh != nil {
		if err := <-errCh; err != nil {
			utils.Debugf("Error hijack: %s", err)
			return err
		}
	}

	// Detached mode: wait for the id to be displayed and return.
	if !config.AttachStdout && !config.AttachStderr {
		// Detached mode
		<-waitDisplayId
		return nil
	}

	var status int

	// Attached mode
	if autoRemove {
		// Autoremove: wait for the container to finish, retrieve
		// the exit code and remove the container
		if _, _, err := readBody(cli.call("POST", "/containers/"+runResult.Get("Id")+"/wait", nil, false)); err != nil {
			return err
		}
		if _, status, err = getExitCode(cli, runResult.Get("Id")); err != nil {
			return err
		}
		if _, _, err := readBody(cli.call("DELETE", "/containers/"+runResult.Get("Id")+"?v=1", nil, false)); err != nil {
			return err
		}
	} else {
		if !config.Tty {
			// In non-tty mode, we can't dettach, so we know we need to wait.
			if status, err = waitForExit(cli, runResult.Get("Id")); err != nil {
				return err
			}
		} else {
			// In TTY mode, there is a race. If the process dies too slowly, the state can be update after the getExitCode call
			// and result in a wrong exit code.
			// No Autoremove: Simply retrieve the exit code
			if _, status, err = getExitCode(cli, runResult.Get("Id")); err != nil {
				return err
			}
		}
	}
	if status != 0 {
		return &utils.StatusError{StatusCode: status}
	}
	return nil
}

func (cli *DockerCli) CmdCp(args ...string) error {
	cmd := cli.Subcmd("cp", "CONTAINER:PATH HOSTPATH", "Copy files/folders from the PATH to the HOSTPATH")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() != 2 {
		cmd.Usage()
		return nil
	}

	var copyData engine.Env
	info := strings.Split(cmd.Arg(0), ":")

	if len(info) != 2 {
		return fmt.Errorf("Error: Path not specified")
	}

	copyData.Set("Resource", info[1])
	copyData.Set("HostPath", cmd.Arg(1))

	stream, statusCode, err := cli.call("POST", "/containers/"+info[0]+"/copy", copyData, false)
	if stream != nil {
		defer stream.Close()
	}
	if err != nil {
		return err
	}

	if statusCode == 200 {
		if err := archive.Untar(stream, copyData.Get("HostPath"), nil); err != nil {
			return err
		}
	}
	return nil
}

func (cli *DockerCli) CmdSave(args ...string) error {
	cmd := cli.Subcmd("save", "IMAGE", "Save an image to a tar archive (streamed to stdout)")
	if err := cmd.Parse(args); err != nil {
		return err
	}

	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	image := cmd.Arg(0)
	if err := cli.stream("GET", "/images/"+image+"/get", nil, cli.out, nil); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) CmdLoad(args ...string) error {
	cmd := cli.Subcmd("load", "", "Load an image from a tar archive on STDIN")
	if err := cmd.Parse(args); err != nil {
		return err
	}

	if cmd.NArg() != 0 {
		cmd.Usage()
		return nil
	}

	if err := cli.stream("POST", "/images/load", cli.in, cli.out, nil); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) call(method, path string, data interface{}, passAuthInfo bool) (io.ReadCloser, int, error) {
	params := bytes.NewBuffer(nil)
	if data != nil {
		if env, ok := data.(engine.Env); ok {
			if err := env.Encode(params); err != nil {
				return nil, -1, err
			}
		} else {
			buf, err := json.Marshal(data)
			if err != nil {
				return nil, -1, err
			}
			if _, err := params.Write(buf); err != nil {
				return nil, -1, err
			}
		}
	}
	// fixme: refactor client to support redirect
	re := regexp.MustCompile("/+")
	path = re.ReplaceAllString(path, "/")

	req, err := http.NewRequest(method, fmt.Sprintf("/v%g%s", api.APIVERSION, path), params)
	if err != nil {
		return nil, -1, err
	}
	if passAuthInfo {
		cli.LoadConfigFile()
		// Resolve the Auth config relevant for this server
		authConfig := cli.configFile.ResolveAuthConfig(auth.IndexServerAddress())
		getHeaders := func(authConfig auth.AuthConfig) (map[string][]string, error) {
			buf, err := json.Marshal(authConfig)
			if err != nil {
				return nil, err
			}
			registryAuthHeader := []string{
				base64.URLEncoding.EncodeToString(buf),
			}
			return map[string][]string{"X-Registry-Auth": registryAuthHeader}, nil
		}
		if headers, err := getHeaders(authConfig); err == nil && headers != nil {
			for k, v := range headers {
				req.Header[k] = v
			}
		}
	}
	req.Header.Set("User-Agent", "Docker-Client/"+VERSION)
	req.Host = cli.addr
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	} else if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}
	dial, err := net.Dial(cli.proto, cli.addr)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, -1, ErrConnectionRefused
		}
		return nil, -1, err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	resp, err := clientconn.Do(req)
	if err != nil {
		clientconn.Close()
		if strings.Contains(err.Error(), "connection refused") {
			return nil, -1, ErrConnectionRefused
		}
		return nil, -1, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, -1, err
		}
		if len(body) == 0 {
			return nil, resp.StatusCode, fmt.Errorf("Error :%s", http.StatusText(resp.StatusCode))
		}
		return nil, resp.StatusCode, fmt.Errorf("Error: %s", bytes.TrimSpace(body))
	}

	wrapper := utils.NewReadCloserWrapper(resp.Body, func() error {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		return clientconn.Close()
	})
	return wrapper, resp.StatusCode, nil
}

func (cli *DockerCli) stream(method, path string, in io.Reader, out io.Writer, headers map[string][]string) error {
	if (method == "POST" || method == "PUT") && in == nil {
		in = bytes.NewReader([]byte{})
	}

	// fixme: refactor client to support redirect
	re := regexp.MustCompile("/+")
	path = re.ReplaceAllString(path, "/")

	req, err := http.NewRequest(method, fmt.Sprintf("/v%g%s", api.APIVERSION, path), in)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Docker-Client/"+VERSION)
	req.Host = cli.addr
	if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}

	if headers != nil {
		for k, v := range headers {
			req.Header[k] = v
		}
	}

	dial, err := net.Dial(cli.proto, cli.addr)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return fmt.Errorf("Can't connect to docker daemon. Is 'docker -d' running on this host?")
		}
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
		return fmt.Errorf("Error: %s", bytes.TrimSpace(body))
	}

	if api.MatchesContentType(resp.Header.Get("Content-Type"), "application/json") {
		return utils.DisplayJSONMessagesStream(resp.Body, out, cli.terminalFd, cli.isTerminal)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	return nil
}

func (cli *DockerCli) hijack(method, path string, setRawTerminal bool, in io.ReadCloser, stdout, stderr io.Writer, started chan io.Closer) error {
	defer func() {
		if started != nil {
			close(started)
		}
	}()
	// fixme: refactor client to support redirect
	re := regexp.MustCompile("/+")
	path = re.ReplaceAllString(path, "/")

	req, err := http.NewRequest(method, fmt.Sprintf("/v%g%s", api.APIVERSION, path), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Docker-Client/"+VERSION)
	req.Header.Set("Content-Type", "plain/text")
	req.Host = cli.addr

	dial, err := net.Dial(cli.proto, cli.addr)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return fmt.Errorf("Can't connect to docker daemon. Is 'docker -d' running on this host?")
		}
		return err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	defer clientconn.Close()

	// Server hijacks the connection, error 'connection closed' expected
	clientconn.Do(req)

	rwc, br := clientconn.Hijack()
	defer rwc.Close()

	if started != nil {
		started <- rwc
	}

	var receiveStdout chan error

	var oldState *term.State

	if in != nil && setRawTerminal && cli.isTerminal && os.Getenv("NORAW") == "" {
		oldState, err = term.SetRawTerminal(cli.terminalFd)
		if err != nil {
			return err
		}
		defer term.RestoreTerminal(cli.terminalFd, oldState)
	}

	if stdout != nil || stderr != nil {
		receiveStdout = utils.Go(func() (err error) {
			defer func() {
				if in != nil {
					if setRawTerminal && cli.isTerminal {
						term.RestoreTerminal(cli.terminalFd, oldState)
					}
					in.Close()
				}
			}()

			// When TTY is ON, use regular copy
			if setRawTerminal {
				_, err = io.Copy(stdout, br)
			} else {
				_, err = utils.StdCopy(stdout, stderr, br)
			}
			utils.Debugf("[hijack] End of stdout")
			return err
		})
	}

	sendStdin := utils.Go(func() error {
		if in != nil {
			io.Copy(rwc, in)
			utils.Debugf("[hijack] End of stdin")
		}
		if tcpc, ok := rwc.(*net.TCPConn); ok {
			if err := tcpc.CloseWrite(); err != nil {
				utils.Errorf("Couldn't send EOF: %s\n", err)
			}
		} else if unixc, ok := rwc.(*net.UnixConn); ok {
			if err := unixc.CloseWrite(); err != nil {
				utils.Errorf("Couldn't send EOF: %s\n", err)
			}
		}
		// Discard errors due to pipe interruption
		return nil
	})

	if stdout != nil || stderr != nil {
		if err := <-receiveStdout; err != nil {
			utils.Errorf("Error receiveStdout: %s", err)
			return err
		}
	}

	if !cli.isTerminal {
		if err := <-sendStdin; err != nil {
			utils.Errorf("Error sendStdin: %s", err)
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
		utils.Errorf("Error getting size: %s", err)
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
	if _, _, err := readBody(cli.call("POST", "/containers/"+id+"/resize?"+v.Encode(), nil, false)); err != nil {
		utils.Errorf("Error resize: %s", err)
	}
}

func (cli *DockerCli) monitorTtySize(id string) error {
	cli.resizeTty(id)

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGWINCH)
	go func() {
		for _ = range sigchan {
			cli.resizeTty(id)
		}
	}()
	return nil
}

func (cli *DockerCli) Subcmd(name, signature, description string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.Usage = func() {
		fmt.Fprintf(cli.err, "\nUsage: docker %s %s\n\n%s\n\n", name, signature, description)
		flags.PrintDefaults()
		os.Exit(2)
	}
	return flags
}

func (cli *DockerCli) LoadConfigFile() (err error) {
	cli.configFile, err = auth.LoadConfig(os.Getenv("HOME"))
	if err != nil {
		fmt.Fprintf(cli.err, "WARNING: %s\n", err)
	}
	return err
}

func waitForExit(cli *DockerCli, containerId string) (int, error) {
	stream, _, err := cli.call("POST", "/containers/"+containerId+"/wait", nil, false)
	if err != nil {
		return -1, err
	}

	var out engine.Env
	if err := out.Decode(stream); err != nil {
		return -1, err
	}
	return out.GetInt("StatusCode"), nil
}

// getExitCode perform an inspect on the container. It returns
// the running state and the exit code.
func getExitCode(cli *DockerCli, containerId string) (bool, int, error) {
	body, _, err := readBody(cli.call("GET", "/containers/"+containerId+"/json", nil, false))
	if err != nil {
		// If we can't connect, then the daemon probably died.
		if err != ErrConnectionRefused {
			return false, -1, err
		}
		return false, -1, nil
	}
	c := &Container{}
	if err := json.Unmarshal(body, c); err != nil {
		return false, -1, err
	}
	return c.State.IsRunning(), c.State.GetExitCode(), nil
}

func readBody(stream io.ReadCloser, statusCode int, err error) ([]byte, int, error) {
	if stream != nil {
		defer stream.Close()
	}
	if err != nil {
		return nil, statusCode, err
	}
	body, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, -1, err
	}
	return body, statusCode, nil
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
	return &DockerCli{
		proto:      proto,
		addr:       addr,
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
