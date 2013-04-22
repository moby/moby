package docker

import (
	_"bytes"
	"encoding/json"
	"flag"
	"fmt"
	_"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"text/tabwriter"
)

const VERSION = "0.1.4"

var (
	GIT_COMMIT      string
	NO_MEMORY_LIMIT bool
)

func ParseCommands(args []string) error {

	cmds := map[string]func(args []string) error{
		"images":  CmdImages,
		"info":    CmdInfo,
		"history": CmdHistory,
		"kill":    CmdKill,
		"logs":    CmdLogs,
		"ps":      CmdPs,
		"restart": CmdRestart,
		"rm":      CmdRm,
		"rmi":     CmdRmi,
		"start":   CmdStart,
		"stop":    CmdStop,
		"version": CmdVersion,
	}

	if len(args) > 0 {
		cmd, exists := cmds[args[0]]
		if !exists {
			//TODO display commend not found
			return cmdHelp(args)
		}
		return cmd(args[1:])
	}
	return cmdHelp(args)
}

func cmdHelp(args []string) error {
	help := "Usage: docker COMMAND [arg...]\n\nA self-sufficient runtime for linux containers.\n\nCommands:\n"
	for _, cmd := range [][]string{
		//		{"attach", "Attach to a running container"},
		//		{"commit", "Create a new image from a container's changes"},
		//		{"diff", "Inspect changes on a container's filesystem"},
		//		{"export", "Stream the contents of a container as a tar archive"},
		{"history", "Show the history of an image"},
		{"images", "List images"},
		//		{"import", "Create a new filesystem image from the contents of a tarball"},
		{"info", "Display system-wide information"},
		//		{"inspect", "Return low-level information on a container"},
		{"kill", "Kill a running container"},
		//		{"login", "Register or Login to the docker registry server"},
		{"logs", "Fetch the logs of a container"},
		//		{"port", "Lookup the public-facing port which is NAT-ed to PRIVATE_PORT"},
		{"ps", "List containers"},
		//		{"pull", "Pull an image or a repository from the docker registry server"},
		//		{"push", "Push an image or a repository to the docker registry server"},
		{"restart", "Restart a running container"},
		{"rm", "Remove a container"},
		{"rmi", "Remove an image"},
		//		{"run", "Run a command in a new container"},
		{"start", "Start a stopped container"},
		{"stop", "Stop a running container"},
		//		{"tag", "Tag an image into a repository"},
		{"version", "Show the docker version information"},
		//		{"wait", "Block until a container stops, then print its exit code"},
	} {
		help += fmt.Sprintf("    %-10.10s%s\n", cmd[0], cmd[1])
	}
	fmt.Println(help)
	return nil
}

/*
// 'docker login': login / register a user to registry service.
func (srv *Server) CmdLogin(stdin io.ReadCloser, stdout rcli.DockerConn, args ...string) error {
	// Read a line on raw terminal with support for simple backspace
	// sequences and echo.
	//
	// This function is necessary because the login command must be done in a
	// raw terminal for two reasons:
	// - we have to read a password (without echoing it);
	// - the rcli "protocol" only supports cannonical and raw modes and you
	//   can't tune it once the command as been started.
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

	stdout.SetOptionRawTerminal()

	cmd := rcli.Subcmd(stdout, "login", "", "Register or Login to the docker registry server")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	var username string
	var password string
	var email string

	fmt.Fprint(stdout, "Username (", srv.runtime.authConfig.Username, "): ")
	username = readAndEchoString(stdin, stdout)
	if username == "" {
		username = srv.runtime.authConfig.Username
	}
	if username != srv.runtime.authConfig.Username {
		fmt.Fprint(stdout, "Password: ")
		password = readString(stdin, stdout)

		if password == "" {
			return fmt.Errorf("Error : Password Required")
		}

		fmt.Fprint(stdout, "Email (", srv.runtime.authConfig.Email, "): ")
		email = readAndEchoString(stdin, stdout)
		if email == "" {
			email = srv.runtime.authConfig.Email
		}
	} else {
		password = srv.runtime.authConfig.Password
		email = srv.runtime.authConfig.Email
	}
	newAuthConfig := auth.NewAuthConfig(username, password, email, srv.runtime.root)
	status, err := auth.Login(newAuthConfig)
	if err != nil {
		fmt.Fprintf(stdout, "Error: %s\r\n", err)
	} else {
		srv.runtime.authConfig = newAuthConfig
	}
	if status != "" {
		fmt.Fprint(stdout, status)
	}
	return nil
}
*/

/*
// 'docker wait': block until a container stops
func (srv *Server) CmdWait(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "wait", "CONTAINER [CONTAINER...]", "Block until a container stops, then print its exit code.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	for _, name := range cmd.Args() {
		if container := srv.runtime.Get(name); container != nil {
			fmt.Fprintln(stdout, container.Wait())
		} else {
			return fmt.Errorf("No such container: %s", name)
		}
	}
	return nil
}
*/


// 'docker version': show version information
func CmdVersion(args []string) error {
	cmd := Subcmd("version", "", "Show the docker version information.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() > 0 {
		cmd.Usage()
		return nil
	}

	body, err := call("GET", "version")
	if err != nil {
		return err
	}

	var out VersionOut
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	fmt.Println("Version:", out.Version)
	fmt.Println("Git Commit:", out.GitCommit)
	if out.MemoryLimitDisabled {
		fmt.Println("Memory limit disabled")
	}

	return nil
}

// 'docker info': display system-wide information.
func CmdInfo(args []string) error {
	cmd := Subcmd("info", "", "Display system-wide information")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() > 0 {
		cmd.Usage()
		return nil
	}

	body, err := call("GET", "info")
	if err != nil {
		return err
	}

	var out InfoOut
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

func CmdStop(args []string) error {
	cmd := Subcmd("stop", "CONTAINER [CONTAINER...]", "Stop a running container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range args {
		_, err := call("GET", "/containers/"+name+"/stop")
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

func CmdRestart(args []string) error {
	cmd := Subcmd("restart", "CONTAINER [CONTAINER...]", "Restart a running container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range args {
		_, err := call("GET", "/containers/"+name+"/restart")
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

func CmdStart(args []string) error {
	cmd := Subcmd("start", "CONTAINER [CONTAINER...]", "Restart a stopped container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range args {
		_, err := call("GET", "/containers/"+name+"/start")
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

/*
func (srv *Server) CmdInspect(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "inspect", "CONTAINER", "Return low-level information on a container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	name := cmd.Arg(0)
	var obj interface{}
	if container := srv.runtime.Get(name); container != nil {
		obj = container
	} else if image, err := srv.runtime.repositories.LookupImage(name); err == nil && image != nil {
		obj = image
	} else {
		// No output means the object does not exist
		// (easier to script since stdout and stderr are not differentiated atm)
		return nil
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	indented := new(bytes.Buffer)
	if err = json.Indent(indented, data, "", "    "); err != nil {
		return err
	}
	if _, err := io.Copy(stdout, indented); err != nil {
		return err
	}
	stdout.Write([]byte{'\n'})
	return nil
}
*/

/*
func (srv *Server) CmdPort(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "port", "CONTAINER PRIVATE_PORT", "Lookup the public-facing port which is NAT-ed to PRIVATE_PORT")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 2 {
		cmd.Usage()
		return nil
	}
	name := cmd.Arg(0)
	privatePort := cmd.Arg(1)
	if container := srv.runtime.Get(name); container == nil {
		return fmt.Errorf("No such container: %s", name)
	} else {
		if frontend, exists := container.NetworkSettings.PortMapping[privatePort]; !exists {
			return fmt.Errorf("No private port '%s' allocated on %s", privatePort, name)
		} else {
			fmt.Fprintln(stdout, frontend)
		}
	}
	return nil
}
*/

// 'docker rmi IMAGE' removes all images with the name IMAGE
func CmdRmi(args []string) error {
	cmd := Subcmd("rmi", "IMAGE [IMAGE...]", "Remove an image")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range args {
		_, err := call("DELETE", "/images/"+name)
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

func CmdHistory(args []string) error {
	cmd := Subcmd("history", "IMAGE", "Show the history of an image")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	body, err := call("GET", "images/"+cmd.Arg(0)+"/history")
	if err != nil {
		return err
	}

	var outs []HistoryOut
	err = json.Unmarshal(body, &outs)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tCREATED\tCREATED BY")

	for _, out := range outs {
		fmt.Fprintf(w, "%s\t%s\t%s\n", out.Id, out.Created, out.CreatedBy)
	}
	w.Flush()
	return nil
}

func CmdRm(args []string) error {
	cmd := Subcmd("rm", "CONTAINER [CONTAINER...]", "Remove a container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range args {
		_, err := call("DELETE", "/containers/"+name)
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

// 'docker kill NAME' kills a running container
func CmdKill(args []string) error {
	cmd := Subcmd("kill", "CONTAINER [CONTAINER...]", "Kill a running container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}

	for _, name := range args {
		_, err := call("POST", "/containers/"+name+"/kill")
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

/*
func (srv *Server) CmdImport(stdin io.ReadCloser, stdout rcli.DockerConn, args ...string) error {
	stdout.Flush()
	cmd := rcli.Subcmd(stdout, "import", "URL|- [REPOSITORY [TAG]]", "Create a new filesystem image from the contents of a tarball")
	var archive io.Reader
	var resp *http.Response

	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	src := cmd.Arg(0)
	if src == "-" {
		archive = stdin
	} else {
		u, err := url.Parse(src)
		if err != nil {
			return err
		}
		if u.Scheme == "" {
			u.Scheme = "http"
			u.Host = src
			u.Path = ""
		}
		fmt.Fprintln(stdout, "Downloading from", u)
		// Download with curl (pretty progress bar)
		// If curl is not available, fallback to http.Get()
		resp, err = Download(u.String(), stdout)
		if err != nil {
			return err
		}
		archive = ProgressReader(resp.Body, int(resp.ContentLength), stdout)
	}
	img, err := srv.runtime.graph.Create(archive, nil, "Imported from "+src)
	if err != nil {
		return err
	}
	// Optionally register the image at REPO/TAG
	if repository := cmd.Arg(1); repository != "" {
		tag := cmd.Arg(2) // Repository will handle an empty tag properly
		if err := srv.runtime.repositories.Set(repository, tag, img.Id, true); err != nil {
			return err
		}
	}
	fmt.Fprintln(stdout, img.ShortId())
	return nil
}
*/

/*
func (srv *Server) CmdPush(stdin io.ReadCloser, stdout rcli.DockerConn, args ...string) error {
	cmd := rcli.Subcmd(stdout, "push", "NAME", "Push an image or a repository to the registry")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	local := cmd.Arg(0)

	if local == "" {
		cmd.Usage()
		return nil
	}

	// If the login failed, abort
	if srv.runtime.authConfig == nil || srv.runtime.authConfig.Username == "" {
		if err := srv.CmdLogin(stdin, stdout, args...); err != nil {
			return err
		}
		if srv.runtime.authConfig == nil || srv.runtime.authConfig.Username == "" {
			return fmt.Errorf("Please login prior to push. ('docker login')")
		}
	}

	var remote string

	tmp := strings.SplitN(local, "/", 2)
	if len(tmp) == 1 {
		return fmt.Errorf(
			"Impossible to push a \"root\" repository. Please rename your repository in <user>/<repo> (ex: %s/%s)",
			srv.runtime.authConfig.Username, local)
	} else {
		remote = local
	}

	Debugf("Pushing [%s] to [%s]\n", local, remote)

	// Try to get the image
	// FIXME: Handle lookup
	// FIXME: Also push the tags in case of ./docker push myrepo:mytag
	//	img, err := srv.runtime.LookupImage(cmd.Arg(0))
	img, err := srv.runtime.graph.Get(local)
	if err != nil {
		Debugf("The push refers to a repository [%s] (len: %d)\n", local, len(srv.runtime.repositories.Repositories[local]))
		// If it fails, try to get the repository
		if localRepo, exists := srv.runtime.repositories.Repositories[local]; exists {
			if err := srv.runtime.graph.PushRepository(stdout, remote, localRepo, srv.runtime.authConfig); err != nil {
				return err
			}
			return nil
		}

		return err
	}
	err = srv.runtime.graph.PushImage(stdout, img, srv.runtime.authConfig)
	if err != nil {
		return err
	}
	return nil
}
*/

/*
func (srv *Server) CmdPull(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "pull", "NAME", "Pull an image or a repository from the registry")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	remote := cmd.Arg(0)
	if remote == "" {
		cmd.Usage()
		return nil
	}

	// FIXME: CmdPull should be a wrapper around Runtime.Pull()
	if srv.runtime.graph.LookupRemoteImage(remote, srv.runtime.authConfig) {
		if err := srv.runtime.graph.PullImage(stdout, remote, srv.runtime.authConfig); err != nil {
			return err
		}
		return nil
	}
	// FIXME: Allow pull repo:tag
	if err := srv.runtime.graph.PullRepository(stdout, remote, "", srv.runtime.repositories, srv.runtime.authConfig); err != nil {
		return err
	}
	return nil
}
*/

func CmdImages(args []string) error {
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
		v.Set("quiet", "true")
	}
	if *all {
		v.Set("all", "true")
	}

	body, err := call("GET", "images?"+v.Encode())
	if err != nil {
		return err
	}

	var outs []ImagesOut
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
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", out.Repository, out.Tag, out.Id, out.Created)
		} else {
			fmt.Fprintln(w, out.Id)
		}
	}

	if !*quiet {
		w.Flush()
	}
	return nil
}

func CmdPs(args []string) error {
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
		v.Set("quiet", "true")
	}
	if *all {
		v.Set("all", "true")
	}
	if *noTrunc {
		v.Set("notrunc", "true")
	}
	if *last != -1 {
		v.Set("n", strconv.Itoa(*last))
	}
	
	body, err := call("GET", "containers?"+v.Encode())
	if err != nil {
		return err
	}

	var outs []PsOut
	err = json.Unmarshal(body, &outs)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	if !*quiet {
		fmt.Fprintln(w, "ID\tIMAGE\tCOMMAND\tCREATED\tSTATUS")
	}

	for _, out := range outs {
		if !*quiet {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", out.Id, out.Image, out.Command, out.Status, out.Created)
		} else {
			fmt.Fprintln(w, out.Id)
		}
	}

	if !*quiet {
		w.Flush()
	}
	return nil
}

/*
func (srv *Server) CmdCommit(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout,
		"commit", "[OPTIONS] CONTAINER [REPOSITORY [TAG]]",
		"Create a new image from a container's changes")
	flComment := cmd.String("m", "", "Commit message")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	containerName, repository, tag := cmd.Arg(0), cmd.Arg(1), cmd.Arg(2)
	if containerName == "" {
		cmd.Usage()
		return nil
	}
	img, err := srv.runtime.Commit(containerName, repository, tag, *flComment)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, img.ShortId())
	return nil
}
*/

/*
func (srv *Server) CmdExport(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout,
		"export", "CONTAINER",
		"Export the contents of a filesystem as a tar archive")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	name := cmd.Arg(0)
	if container := srv.runtime.Get(name); container != nil {
		data, err := container.Export()
		if err != nil {
			return err
		}
		// Stream the entire contents of the container (basically a volatile snapshot)
		if _, err := io.Copy(stdout, data); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("No such container: %s", name)
}
*/

/*
func (srv *Server) CmdDiff(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout,
		"diff", "CONTAINER",
		"Inspect changes on a container's filesystem")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	if container := srv.runtime.Get(cmd.Arg(0)); container == nil {
		return fmt.Errorf("No such container")
	} else {
		changes, err := container.Changes()
		if err != nil {
			return err
		}
		for _, change := range changes {
			fmt.Fprintln(stdout, change.String())
		}
	}
	return nil
}
*/

func CmdLogs(args []string) error {
	cmd := Subcmd("logs", "CONTAINER", "Fetch the logs of a container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}
	body, err := call("GET", "containers/"+cmd.Arg(0)+"/logs")
	if err != nil {
		return err
	}

	var out LogsOut
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, out.Stdout)
	fmt.Fprintln(os.Stderr, out.Stderr)

	return nil
}

/*
func (srv *Server) CmdAttach(stdin io.ReadCloser, stdout rcli.DockerConn, args ...string) error {
	cmd := rcli.Subcmd(stdout, "attach", "CONTAINER", "Attach to a running container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}
	name := cmd.Arg(0)
	container := srv.runtime.Get(name)
	if container == nil {
		return fmt.Errorf("No such container: %s", name)
	}

	if container.Config.Tty {
		stdout.SetOptionRawTerminal()
	}
	// Flush the options to make sure the client sets the raw mode
	stdout.Flush()
	return <-container.Attach(stdin, nil, stdout, stdout)
}
*/
/*
// Ports type - Used to parse multiple -p flags
type ports []int

func (p *ports) String() string {
	return fmt.Sprint(*p)
}

func (p *ports) Set(value string) error {
	port, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("Invalid port: %v", value)
	}
	*p = append(*p, port)
	return nil
}
*/

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


/*
func (srv *Server) CmdTag(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "tag", "[OPTIONS] IMAGE REPOSITORY [TAG]", "Tag an image into a repository")
	force := cmd.Bool("f", false, "Force")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 2 {
		cmd.Usage()
		return nil
	}
	return srv.runtime.repositories.Set(cmd.Arg(1), cmd.Arg(2), cmd.Arg(0), *force)
}
*/

/*
func (srv *Server) CmdRun(stdin io.ReadCloser, stdout rcli.DockerConn, args ...string) error {
	config, err := ParseRun(args)
	if err != nil {
		return err
	}
	if config.Image == "" {
		fmt.Fprintln(stdout, "Error: Image not specified")
		return fmt.Errorf("Image not specified")
	}
	if len(config.Cmd) == 0 {
		fmt.Fprintln(stdout, "Error: Command not specified")
		return fmt.Errorf("Command not specified")
	}

	if config.Tty {
		stdout.SetOptionRawTerminal()
	}
	// Flush the options to make sure the client sets the raw mode
	// or tell the client there is no options
	stdout.Flush()

	// Create new container
	container, err := srv.runtime.Create(config)
	if err != nil {
		// If container not found, try to pull it
		if srv.runtime.graph.IsNotExist(err) {
			fmt.Fprintf(stdout, "Image %s not found, trying to pull it from registry.\r\n", config.Image)
			if err = srv.CmdPull(stdin, stdout, config.Image); err != nil {
				return err
			}
			if container, err = srv.runtime.Create(config); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	var (
		cStdin           io.ReadCloser
		cStdout, cStderr io.Writer
	)
	if config.AttachStdin {
		r, w := io.Pipe()
		go func() {
			defer w.Close()
			defer Debugf("Closing buffered stdin pipe")
			io.Copy(w, stdin)
		}()
		cStdin = r
	}
	if config.AttachStdout {
		cStdout = stdout
	}
	if config.AttachStderr {
		cStderr = stdout // FIXME: rcli can't differentiate stdout from stderr
	}

	attachErr := container.Attach(cStdin, stdin, cStdout, cStderr)
	Debugf("Starting\n")
	if err := container.Start(); err != nil {
		return err
	}
	if cStdout == nil && cStderr == nil {
		fmt.Fprintln(stdout, container.ShortId())
	}
	Debugf("Waiting for attach to return\n")
	<-attachErr
	// Expecting I/O pipe error, discarding
	return nil
 }
 */

	
func call(method, path string) ([]byte, error) {
	req, err := http.NewRequest(method, "http://0.0.0.0:4243/" + path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error: %s", body)
	}
	return body, nil

}
/*
func apiPost(path string, data interface{}) ([]byte, error) {
	buf, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	dataBuf := bytes.NewBuffer(buf)
	resp, err := http.Post("http://0.0.0.0:4243/"+path, "application/json", dataBuf)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("[error] %s", body)
	}
	return body, nil
}

func apiPostHijack(path string, data interface{}) (io.ReadCloser, error) {
	buf, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	dataBuf := bytes.NewBuffer(buf)
	resp, err := http.Post("http://0.0.0.0:4243/"+path, "application/json", dataBuf)
	if err != nil {
		return nil, err
	}
	//TODO check status code

	return resp.Body, nil
}
*/
func Subcmd(name, signature, description string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.Usage = func() {
		fmt.Printf("\nUsage: docker %s %s\n\n%s\n\n", name, signature, description)
		flags.PrintDefaults()
	}
	return flags
}
