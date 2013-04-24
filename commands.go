package docker

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
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
		"commit":  CmdCommit,
		"diff":    CmdDiff,
		"export":  CmdExport,
		"images":  CmdImages,
		"info":    CmdInfo,
		"inspect": CmdInspect,
		"history": CmdHistory,
		"kill":    CmdKill,
		"logs":    CmdLogs,
		"port":    CmdPort,
		"ps":      CmdPs,
		"pull":    CmdPull,
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
		{"commit", "Create a new image from a container's changes"},
		{"diff", "Inspect changes on a container's filesystem"},
		{"export", "Stream the contents of a container as a tar archive"},
		{"history", "Show the history of an image"},
		{"images", "List images"},
		//		{"import", "Create a new filesystem image from the contents of a tarball"},
		{"info", "Display system-wide information"},
		{"inspect", "Return low-level information on a container/image"},
		{"kill", "Kill a running container"},
		//		{"login", "Register or Login to the docker registry server"},
		{"logs", "Fetch the logs of a container"},
		{"port", "Lookup the public-facing port which is NAT-ed to PRIVATE_PORT"},
		{"ps", "List containers"},
		{"pull", "Pull an image or a repository from the docker registry server"},
		//		{"push", "Push an image or a repository to the docker registry server"},
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

// 'docker wait': block until a container stops
func CmdWait(args []string) error {
	cmd := Subcmd("wait", "CONTAINER [CONTAINER...]", "Block until a container stops, then print its exit code.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	for _, name := range cmd.Args() {
		body, err := call("POST", "/containers/"+name+"/wait")
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
func CmdVersion(args []string) error {
	cmd := Subcmd("version", "", "Show the docker version information.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() > 0 {
		cmd.Usage()
		return nil
	}

	body, err := call("GET", "/version")
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

	body, err := call("GET", "/info")
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
		_, err := call("POST", "/containers/"+name+"/stop")
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
		_, err := call("POST", "/containers/"+name+"/restart")
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
		_, err := call("POST", "/containers/"+name+"/start")
		if err != nil {
			fmt.Printf("%s", err)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

func CmdInspect(args []string) error {
	cmd := Subcmd("inspect", "CONTAINER|IMAGE", "Return low-level information on a container/image")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}
	var obj interface{}
	var err error
	obj, err = call("GET", "/containers/"+cmd.Arg(0))
	if err != nil {
		obj, err = call("GET", "/images/"+cmd.Arg(0))
		if err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", b)
	return nil
}

func CmdPort(args []string) error {
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
	body, err := call("GET", "/containers/"+cmd.Arg(0)+"/port?"+v.Encode())
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

	body, err := call("GET", "/images/"+cmd.Arg(0)+"/history")
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

func CmdPull(args []string) error {
	cmd := Subcmd("pull", "NAME", "Pull an image or a repository from the registry")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	if err := callStream("POST", "/images/"+cmd.Arg(0)+"/pull", nil, false); err != nil {
		return err
	}

	return nil
}

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

	body, err := call("GET", "/images?"+v.Encode())
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

	body, err := call("GET", "/containers?"+v.Encode())
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
		fmt.Fprintln(w, "ID\tIMAGE\tCOMMAND\tCREATED\tSTATUS")
	}

	for _, out := range outs {
		if !*quiet {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", out.Id, out.Image, out.Command, out.Status, out.Created)
		} else {
			fmt.Fprintln(w, out.Id)
		}
	}

	if !*quiet {
		w.Flush()
	}
	return nil
}

func CmdCommit(args []string) error {
	cmd := Subcmd("commit", "[OPTIONS] CONTAINER [REPOSITORY [TAG]]", "Create a new image from a container's changes")
	flComment := cmd.String("m", "", "Commit message")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	name, repository, tag := cmd.Arg(0), cmd.Arg(1), cmd.Arg(2)
	if name == "" {
		cmd.Usage()
		return nil
	}
	v := url.Values{}
	v.Set("repo", repository)
	v.Set("tag", tag)
	v.Set("comment", *flComment)

	body, err := call("POST", "/containers/"+name+"/commit?"+v.Encode())
	if err != nil {
		return err
	}

	var out ApiCommit
	err = json.Unmarshal(body, &out)
	if err != nil {
		return err
	}

	fmt.Println(out.Id)
	return nil
}

func CmdExport(args []string) error {
	cmd := Subcmd("export", "CONTAINER", "Export the contents of a filesystem as a tar archive")
	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	if err := callStream("GET", "/containers/"+cmd.Arg(0)+"/export", nil, false); err != nil {
		return err
	}
	return nil
}

func CmdDiff(args []string) error {
	cmd := Subcmd("diff", "CONTAINER", "Inspect changes on a container's filesystem")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}

	body, err := call("GET", "/containers/"+cmd.Arg(0)+"/changes")
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

func CmdLogs(args []string) error {
	cmd := Subcmd("logs", "CONTAINER", "Fetch the logs of a container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}
	body, err := call("GET", "/containers/"+cmd.Arg(0)+"/logs")
	if err != nil {
		return err
	}

	var out ApiLogs
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

func CmdTag(args []string) error {
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
		v.Set("force", "true")
	}

	if err := callStream("POST", "/images/"+cmd.Arg(0)+"/tag?"+v.Encode(), nil, false); err != nil {
		return err
	}
	return nil
}

func CmdRun(args []string) error {
	fmt.Println("CmdRun")
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

	if err := callStream("POST", "/containers", *config, config.Tty); err != nil {
		return err
	}
	return nil
}

func call(method, path string) ([]byte, error) {
	req, err := http.NewRequest(method, "http://0.0.0.0:4243"+path, nil)
	if err != nil {
		return nil, err
	}
	if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
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

func callStream(method, path string, data interface{}, isTerminal bool) error {
	var body io.Reader
	if data != nil {
		buf, err := json.Marshal(data)
		if err != nil {
			return err
		}
		body = bytes.NewBuffer(buf)
	}
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		return err
	}

	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	dial, err := net.Dial("tcp", "0.0.0.0:4243")
	if err != nil {
		return err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	resp, err := clientconn.Do(req)
	defer clientconn.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("error: %s", body)
	}

	rwc, _ := clientconn.Hijack()
	defer rwc.Close()

	receiveStdout := Go(func() error {
		_, err := io.Copy(os.Stdout, rwc)
		return err
	})
	sendStdin := Go(func() error {
		_, err := io.Copy(rwc, os.Stdin)
		rwc.Close()
		return err
	})

	if err := <-receiveStdout; err != nil {
		return err
	}
	if isTerminal {
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
