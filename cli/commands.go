package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/auth"
	"github.com/dotcloud/docker/client"
	"github.com/dotcloud/docker/core"
	"github.com/dotcloud/docker/server"
	"github.com/dotcloud/docker/term"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"
	"unicode"
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
	help := fmt.Sprintf("Usage: docker [OPTIONS] COMMAND [arg...]\n  -H=[tcp://%s:%d]: tcp://host:port to bind/connect to or unix://path/to/socket to use\n\nA self-sufficient runtime for linux containers.\n\nCommands:\n", server.DEFAULTHTTPHOST, server.DEFAULTHTTPPORT)
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
	cmd := core.Subcmd("insert", "IMAGE URL PATH", "Insert a file from URL in the IMAGE at PATH")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 3 {
		cmd.Usage()
		return nil
	}

	return cli.client.ImageInsert(cmd.Arg(0), cmd.Arg(1), cmd.Arg(2), utils.JSONMessageStreamFormatter(cli.out))
}

func (cli *DockerCli) CmdBuild(args ...string) error {
	cmd := core.Subcmd("build", "[OPTIONS] PATH | URL | -", "Build a new container image from the source code at PATH")
	tag := cmd.String("t", "", "Repository name (and optionally a tag) to be applied to the resulting image in case of success")
	suppressOutput := cmd.Bool("q", false, "Suppress verbose build output")
	noCache := cmd.Bool("no-cache", false, "Do not use cache when building the image")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	dockerfile := cmd.Arg(0)
	if dockerfile == "-" {
		// As a special case, 'docker build -' will build from an empty context with the
		// contents of stdin as a Dockerfile
		d, err := ioutil.ReadAll(cli.in)
		if err != nil {
			return err
		}

		dockerfile = string(d)
	}

	return cli.client.Build(dockerfile, *tag, *suppressOutput, *noCache, cli.out, cli.err)
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

	cmd := core.Subcmd("login", "[OPTIONS]", "Register or Login to the docker registry server")
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

	var promptDefault = func(prompt string, configDefault string) {
		if configDefault == "" {
			fmt.Fprintf(cli.out, "%s: ", prompt)
		} else {
			fmt.Fprintf(cli.out, "%s (%s): ", prompt, configDefault)
		}
	}

	authconfig, ok := cli.client.Config(auth.IndexServerAddress())
	if !ok {
		authconfig = auth.AuthConfig{}
	}

	if *flUsername == "" {
		promptDefault("Username", authconfig.Username)
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
			promptDefault("Email", authconfig.Email)
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
	status, err := cli.client.Authenticate(username, password, email)
	if err != nil {
		return err
	}

	if status != "" {
		fmt.Fprintf(cli.out, "%s\n", status)
	}
	return nil
}

// 'docker wait': block until a container stops
func (cli *DockerCli) CmdWait(args ...string) error {
	cmd := core.Subcmd("wait", "CONTAINER [CONTAINER...]", "Block until a container stops, then print its exit code.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	for _, name := range cmd.Args() {
		statusCode, err := cli.client.ContainerWait(name)
		if err != nil {
			fmt.Fprintf(cli.err, "%s", err)
		} else {
			fmt.Fprintf(cli.out, "%d\n", statusCode)
		}
	}
	return nil
}

// 'docker version': show version information
func (cli *DockerCli) CmdVersion(args ...string) error {
	cmd := core.Subcmd("version", "", "Show the docker version information.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() > 0 {
		cmd.Usage()
		return nil
	}

	vers, err := cli.client.Version()
	if err != nil {
		return err
	}

	fmt.Fprintf(cli.out, "Client version: %s\n", client.VERSION)
	fmt.Fprintf(cli.out, "Server version: %s\n", vers.Version)
	if vers.GitCommit != "" {
		fmt.Fprintf(cli.out, "Git commit: %s\n", vers.GitCommit)
	}
	if vers.GoVersion != "" {
		fmt.Fprintf(cli.out, "Go version: %s\n", vers.GoVersion)
	}

	release := utils.GetReleaseVersion()
	if release != "" {
		fmt.Fprintf(cli.out, "Last stable version: %s", release)
		if strings.Trim(client.VERSION, "-dev") != release || strings.Trim(vers.Version, "-dev") != release {
			fmt.Fprintf(cli.out, ", please update docker")
		}
		fmt.Fprintf(cli.out, "\n")
	}
	return nil
}

// 'docker info': display system-wide information.
func (cli *DockerCli) CmdInfo(args ...string) error {
	cmd := core.Subcmd("info", "", "Display system-wide information")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() > 0 {
		cmd.Usage()
		return nil
	}

	info, err := cli.client.Info()
	if err != nil {
		return err
	}

	fmt.Fprintf(cli.out, "Containers: %d\n", info.Containers)
	fmt.Fprintf(cli.out, "Images: %d\n", info.Images)
	if info.Debug || os.Getenv("DEBUG") != "" {
		fmt.Fprintf(cli.out, "Debug mode (server): %v\n", info.Debug)
		fmt.Fprintf(cli.out, "Debug mode (client): %v\n", os.Getenv("DEBUG") != "")
		fmt.Fprintf(cli.out, "Fds: %d\n", info.NFd)
		fmt.Fprintf(cli.out, "Goroutines: %d\n", info.NGoroutines)
		fmt.Fprintf(cli.out, "LXC Version: %s\n", info.LXCVersion)
		fmt.Fprintf(cli.out, "EventsListeners: %d\n", info.NEventsListener)
		fmt.Fprintf(cli.out, "Kernel Version: %s\n", info.KernelVersion)
	}

	if len(info.IndexServerAddress) != 0 {
		if conf, ok := cli.client.Config(info.IndexServerAddress); ok {
			u := conf.Username
			if len(u) > 0 {
				fmt.Fprintf(cli.out, "Username: %v\n", u)
				fmt.Fprintf(cli.out, "Registry: %v\n", info.IndexServerAddress)
			}
		}
	}
	if !info.MemoryLimit {
		fmt.Fprintf(cli.err, "WARNING: No memory limit support\n")
	}
	if !info.SwapLimit {
		fmt.Fprintf(cli.err, "WARNING: No swap limit support\n")
	}
	if !info.IPv4Forwarding {
		fmt.Fprintf(cli.err, "WARNING: IPv4 forwarding is disabled.\n")
	}
	return nil
}

func (cli *DockerCli) CmdStop(args ...string) error {
	cmd := core.Subcmd("stop", "[OPTIONS] CONTAINER [CONTAINER...]", "Stop a running container")
	nSeconds := cmd.Int("t", 10, "Number of seconds to wait for the container to stop before killing it.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range cmd.Args() {
		err := cli.client.ContainerStop(name, *nSeconds)
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return nil
}

func (cli *DockerCli) CmdRestart(args ...string) error {
	cmd := core.Subcmd("restart", "[OPTIONS] CONTAINER [CONTAINER...]", "Restart a running container")
	nSeconds := cmd.Int("t", 10, "Number of seconds to try to stop for before killing the container. Once killed it will then be restarted. Default=10")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range cmd.Args() {
		err := cli.client.ContainerRestart(name, *nSeconds)
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return nil
}

func (cli *DockerCli) CmdStart(args ...string) error {
	cmd := core.Subcmd("start", "CONTAINER [CONTAINER...]", "Restart a stopped container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range args {
		err := cli.client.ContainerStart(name, nil)
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return nil
}

func (cli *DockerCli) CmdInspect(args ...string) error {
	cmd := core.Subcmd("inspect", "CONTAINER|IMAGE [CONTAINER|IMAGE...]", "Return low-level information on a container/image")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	fmt.Fprintf(cli.out, "[")
	for i, name := range args {
		var obj interface{}
		if i > 0 {
			fmt.Fprintf(cli.out, ",")
		}
		obj, err := cli.client.ContainerInspect(name)
		if err != nil {
			obj, err = cli.client.ImageInspect(name)
			if err != nil {
				fmt.Fprintf(cli.err, "%s\n", err)
				continue
			}
		}

		indented := new(bytes.Buffer)
		str, err := json.Marshal(obj)

		if err != nil {
			return err
		}

		if err = json.Indent(indented, str, "", "    "); err != nil {
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
	cmd := core.Subcmd("top", "CONTAINER", "Lookup the running processes of a container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() == 0 {
		cmd.Usage()
		return nil
	}

	procs, err := cli.client.ContainerTop(cmd.Arg(0), cmd.Args()[1:]...)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	fmt.Fprintln(w, strings.Join(procs.Titles, "\t"))
	for _, proc := range procs.Processes {
		fmt.Fprintln(w, strings.Join(proc, "\t"))
	}
	w.Flush()
	return nil
}

func (cli *DockerCli) CmdPort(args ...string) error {
	cmd := core.Subcmd("port", "CONTAINER PRIVATE_PORT", "Lookup the public-facing port which is NAT-ed to PRIVATE_PORT")
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
	container, err := cli.client.ContainerInspect(cmd.Arg(0))
	if err != nil {
		return err
	}

	if frontend, exists := container.NetworkSettings.PortMapping[proto][port]; exists {
		fmt.Fprintf(cli.out, "%s\n", frontend)
	} else {
		return fmt.Errorf("Error: No private port '%s' allocated on %s", cmd.Arg(1), cmd.Arg(0))
	}
	return nil
}

// 'docker rmi IMAGE' removes all images with the name IMAGE
func (cli *DockerCli) CmdRmi(args ...string) error {
	cmd := core.Subcmd("rmi", "IMAGE [IMAGE...]", "Remove one or more images")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range cmd.Args() {
		outs, err := cli.client.ImageRemove(name)
		if err != nil {
			fmt.Fprintf(cli.err, "%s", err)
		} else {
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
	cmd := core.Subcmd("history", "IMAGE", "Show the history of an image")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	outs, err := cli.client.ImageHistory(cmd.Arg(0))
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
	cmd := core.Subcmd("rm", "[OPTIONS] CONTAINER [CONTAINER...]", "Remove one or more containers")
	v := cmd.Bool("v", false, "Remove the volumes associated to the container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	for _, name := range cmd.Args() {
		err := cli.client.ContainerRemove(name, *v)
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
	cmd := core.Subcmd("kill", "CONTAINER [CONTAINER...]", "Kill a running container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range args {
		err := cli.client.ContainerKill(name)
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	return nil
}

func (cli *DockerCli) CmdImport(args ...string) error {
	cmd := core.Subcmd("import", "URL|- [REPOSITORY [TAG]]", "Create a new filesystem image from the contents of a tarball(.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz).")

	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	src, repository, tag := cmd.Arg(0), cmd.Arg(1), cmd.Arg(2)

	return cli.client.ImageCreate("", src, repository, tag, "", utils.JSONMessageStreamFormatter(cli.out), cli.in)
}

func (cli *DockerCli) CmdPush(args ...string) error {
	cmd := core.Subcmd("push", "NAME", "Push an image or a repository to the registry")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	name := cmd.Arg(0)

	if name == "" {
		cmd.Usage()
		return nil
	}

	return cli.client.ImagePush(name, utils.JSONMessageStreamFormatter(cli.out))
}

func (cli *DockerCli) CmdPull(args ...string) error {
	cmd := core.Subcmd("pull", "NAME", "Pull an image or a repository from the registry")
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

	return cli.client.ImageCreate(remote, "", "", *tag, "", utils.JSONMessageStreamFormatter(cli.out), nil)
}

func (cli *DockerCli) CmdImages(args ...string) error {
	cmd := core.Subcmd("images", "[OPTIONS] [NAME]", "List images")
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
		body, err := cli.client.ImageListViz()
		if err != nil {
			return err
		}
		fmt.Fprintf(cli.out, "%s", body)
	} else {
		var filter = ""
		if cmd.NArg() == 1 {
			filter = cmd.Arg(0)
		}

		outs, err := cli.client.ImageList(*all, filter)
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
	cmd := core.Subcmd("ps", "[OPTIONS]", "List containers")
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
	if *last == -1 && *nLatest {
		*last = 1
	}

	outs, err := cli.client.ContainerList(*size, *all, *last, *since, *before)
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
	cmd := core.Subcmd("commit", "[OPTIONS] CONTAINER [REPOSITORY [TAG]]", "Create a new image from a container's changes")
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

	var config *core.Config
	if *flConfig != "" {
		config = &core.Config{}
		if err := json.Unmarshal([]byte(*flConfig), config); err != nil {
			return err
		}
	}
	id, err := cli.client.Commit(name, repository, tag, *flComment, *flAuthor, config)

	if err != nil {
		return err
	}

	fmt.Fprintf(cli.out, "%s\n", id)
	return nil
}

func (cli *DockerCli) CmdEvents(args ...string) error {
	cmd := core.Subcmd("events", "[OPTIONS]", "Get real time events from the server")
	since := cmd.String("since", "", "Show events previously created (used for polling).")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() != 0 {
		cmd.Usage()
		return nil
	}

	return cli.client.Events(*since, utils.JSONMessageStreamFormatter(cli.out))
}

func (cli *DockerCli) CmdExport(args ...string) error {
	cmd := core.Subcmd("export", "CONTAINER", "Export the contents of a filesystem as a tar archive")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	return cli.client.ContainerExport(cmd.Arg(0), cli.out)
}

func (cli *DockerCli) CmdDiff(args ...string) error {
	cmd := core.Subcmd("diff", "CONTAINER", "Inspect changes on a container's filesystem")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	changes, err := cli.client.ContainerDiff(cmd.Arg(0))
	if err != nil {
		return err
	}

	for _, change := range changes {
		fmt.Fprintf(cli.out, "%s\n", change.String())
	}
	return nil
}

func (cli *DockerCli) CmdLogs(args ...string) error {
	cmd := core.Subcmd("logs", "CONTAINER", "Fetch the logs of a container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	return cli.client.ContainerAttach(cmd.Arg(0), true, false, false, true, true, nil, cli.out, nil)
}

func (cli *DockerCli) CmdAttach(args ...string) error {
	cmd := core.Subcmd("attach", "CONTAINER", "Attach to a running container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	container, err := cli.client.ContainerInspect(cmd.Arg(0))
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

	var t *uintptr
	if cli.in != nil && container.Config.Tty {
		t = cli.terminal()
	}

	return cli.client.ContainerAttach(cmd.Arg(0), false, true, true, true, true, cli.in, cli.out, t)
}

func (cli *DockerCli) CmdSearch(args ...string) error {
	cmd := core.Subcmd("search", "NAME", "Search the docker index for images")
	noTrunc := cmd.Bool("notrunc", false, "Don't truncate output")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	outs, err := cli.client.ImageSearch(cmd.Arg(0))
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


func (cli *DockerCli) CmdTag(args ...string) error {
	cmd := core.Subcmd("tag", "[OPTIONS] IMAGE REPOSITORY [TAG]", "Tag an image into a repository")
	force := cmd.Bool("f", false, "Force")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 2 && cmd.NArg() != 3 {
		cmd.Usage()
		return nil
	}

	tag := ""
	if cmd.NArg() == 3 {
		tag = cmd.Arg(2)
	}

	return cli.client.ImageTag(cmd.Arg(0), cmd.Arg(1), tag, *force)
}

func (cli *DockerCli) CmdRun(args ...string) error {
	config, hostConfig, cmd, err := core.ParseRun(args, nil)
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
	runResult, err := cli.client.ContainerCreate(config)

	//if image not found try to pull it
	if e2, ok := err.(*client.APIError); ok && e2.StatusCode == 404 {
		repos, tag := utils.ParseRepositoryTag(config.Image)
		err = cli.client.ImageCreate(repos, "", "", tag, "", utils.JSONMessageStreamFormatter(cli.err), nil)
		if err != nil {
			return err
		}
		runResult, err = cli.client.ContainerCreate(config)
		if err != nil {
			return err
		}
	}
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
	if err = cli.client.ContainerStart(runResult.ID, hostConfig); err != nil {
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

		var t *uintptr
		if cli.in != nil && config.Tty {
			t = cli.terminal()
		}

		if err := cli.client.ContainerAttach(runResult.ID, true, true, config.AttachStdin, config.AttachStdout, config.AttachStderr, cli.in, cli.out, t); err != nil {
			utils.Debugf("Error hijack: %s", err)
			return err
		}
	}

	if !config.AttachStdout && !config.AttachStderr {
		<-wait
	}
	return nil
}

func (cli *DockerCli) CmdCp(args ...string) error {
	cmd := core.Subcmd("cp", "CONTAINER:RESOURCE HOSTPATH", "Copy files/folders from the RESOURCE to the HOSTPATH")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() != 2 {
		cmd.Usage()
		return nil
	}

	info := strings.Split(cmd.Arg(0), ":")

	data, err := cli.client.ContainerCopy(info[0], info[1])
	if err != nil {
		return err
	}

	r := bytes.NewReader(data)
	if err := utils.Untar(r, cmd.Arg(1)); err != nil {
		return err
	}

	return nil
}

func (cli *DockerCli) terminal() *uintptr {
	if cli.isTerminal && os.Getenv("NORAW") == "" {
		return &cli.terminalFd
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
	if err := cli.client.ContainerResize(id, width, height); err != nil {
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
		for _ = range sigchan {
			cli.resizeTty(id)
		}
	}()
	return nil
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
		client:     client.NewClient(proto, addr),
		in:         in,
		out:        out,
		err:        err,
		isTerminal: isTerminal,
		terminalFd: terminalFd,
	}
}

type DockerCli struct {
	client     *client.Client
	in         io.ReadCloser
	out        io.Writer
	err        io.Writer
	isTerminal bool
	terminalFd uintptr
}
