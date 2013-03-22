package docker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dotcloud/docker/auth"
	"github.com/dotcloud/docker/rcli"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
)

const VERSION = "0.0.3"

func (srv *Server) Name() string {
	return "docker"
}

// FIXME: Stop violating DRY by repeating usage here and in Subcmd declarations
func (srv *Server) Help() string {
	help := "Usage: docker COMMAND [arg...]\n\nA self-sufficient runtime for linux containers.\n\nCommands:\n"
	for _, cmd := range [][]interface{}{
		{"run", "Run a command in a container"},
		{"ps", "Display a list of containers"},
		{"import", "Create a new filesystem image from the contents of a tarball"},
		{"attach", "Attach to a running container"},
		{"commit", "Create a new image from a container's changes"},
		{"diff", "Inspect changes on a container's filesystem"},
		{"images", "List images"},
		{"info", "Display system-wide information"},
		{"inspect", "Return low-level information on a container"},
		{"kill", "Kill a running container"},
		{"login", "Register or Login to the docker registry server"},
		{"logs", "Fetch the logs of a container"},
		{"port", "Lookup the public-facing port which is NAT-ed to PRIVATE_PORT"},
		{"ps", "List containers"},
		{"restart", "Restart a running container"},
		{"rm", "Remove a container"},
		{"rmi", "Remove an image"},
		{"run", "Run a command in a new container"},
		{"start", "Start a stopped container"},
		{"stop", "Stop a running container"},
		{"export", "Stream the contents of a container as a tar archive"},
		{"version", "Show the docker version information"},
		{"wait", "Block until a container stops, then print its exit code"},
	} {
		help += fmt.Sprintf("    %-10.10s%s\n", cmd[0], cmd[1])
	}
	return help
}

// 'docker login': login / register a user to registry service.
func (srv *Server) CmdLogin(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "login", "", "Register or Login to the docker registry server")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	var username string
	var password string
	var email string
	authConfig, err := auth.LoadConfig()
	if err != nil {
		fmt.Fprintf(stdout, "Error : %s\n", err)
	}

	fmt.Fprint(stdout, "Username (", authConfig.Username, "): ")
	fmt.Fscanf(stdin, "%s", &username)
	if username == "" {
		username = authConfig.Username
	}
	if username != authConfig.Username {
		fmt.Fprint(stdout, "Password: ")
		fmt.Fscanf(stdin, "%s", &password)

		if password == "" {
			return errors.New("Error : Password Required\n")
		}

		fmt.Fprint(stdout, "Email (", authConfig.Email, "): ")
		fmt.Fscanf(stdin, "%s", &email)
		if email == "" {
			email = authConfig.Email
		}
	} else {
		password = authConfig.Password
		email = authConfig.Email
	}
	newAuthConfig := auth.AuthConfig{Username: username, Password: password, Email: email}
	status, err := auth.Login(newAuthConfig)
	if err != nil {
		fmt.Fprintf(stdout, "Error : %s\n", err)
	}
	if status != "" {
		fmt.Fprintf(stdout, status)
	}
	return nil
}

// 'docker wait': block until a container stops
func (srv *Server) CmdWait(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "wait", "[OPTIONS] NAME", "Block until a container stops, then print its exit code.")
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
			return errors.New("No such container: " + name)
		}
	}
	return nil
}

// 'docker version': show version information
func (srv *Server) CmdVersion(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	fmt.Fprintf(stdout, "Version:%s\n", VERSION)
	return nil
}

// 'docker info': display system-wide information.
func (srv *Server) CmdInfo(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	images, _ := srv.runtime.graph.All()
	var imgcount int
	if images == nil {
		imgcount = 0
	} else {
		imgcount = len(images)
	}
	cmd := rcli.Subcmd(stdout, "info", "", "Display system-wide information.")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() > 0 {
		cmd.Usage()
		return nil
	}
	fmt.Fprintf(stdout, "containers: %d\nversion: %s\nimages: %d\n",
		len(srv.runtime.List()),
		VERSION,
		imgcount)
	return nil
}

func (srv *Server) CmdStop(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "stop", "[OPTIONS] NAME", "Stop a running container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	for _, name := range cmd.Args() {
		if container := srv.runtime.Get(name); container != nil {
			if err := container.Stop(); err != nil {
				return err
			}
			fmt.Fprintln(stdout, container.Id)
		} else {
			return errors.New("No such container: " + name)
		}
	}
	return nil
}

func (srv *Server) CmdRestart(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "restart", "[OPTIONS] NAME", "Restart a running container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	for _, name := range cmd.Args() {
		if container := srv.runtime.Get(name); container != nil {
			if err := container.Restart(); err != nil {
				return err
			}
			fmt.Fprintln(stdout, container.Id)
		} else {
			return errors.New("No such container: " + name)
		}
	}
	return nil
}

func (srv *Server) CmdStart(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "start", "[OPTIONS] NAME", "Start a stopped container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	for _, name := range cmd.Args() {
		if container := srv.runtime.Get(name); container != nil {
			if err := container.Start(); err != nil {
				return err
			}
			fmt.Fprintln(stdout, container.Id)
		} else {
			return errors.New("No such container: " + name)
		}
	}
	return nil
}

func (srv *Server) CmdMount(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "umount", "[OPTIONS] NAME", "mount a container's filesystem (debug only)")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	for _, name := range cmd.Args() {
		if container := srv.runtime.Get(name); container != nil {
			if err := container.EnsureMounted(); err != nil {
				return err
			}
			fmt.Fprintln(stdout, container.Id)
		} else {
			return errors.New("No such container: " + name)
		}
	}
	return nil
}

func (srv *Server) CmdInspect(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "inspect", "[OPTIONS] CONTAINER", "Return low-level information on a container")
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
	} else if image, err := srv.runtime.graph.Get(name); err != nil {
		return err
	} else if image != nil {
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

func (srv *Server) CmdPort(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "port", "[OPTIONS] CONTAINER PRIVATE_PORT", "Lookup the public-facing port which is NAT-ed to PRIVATE_PORT")
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
		return errors.New("No such container: " + name)
	} else {
		if frontend, exists := container.NetworkSettings.PortMapping[privatePort]; !exists {
			return fmt.Errorf("No private port '%s' allocated on %s", privatePort, name)
		} else {
			fmt.Fprintln(stdout, frontend)
		}
	}
	return nil
}

// 'docker rmi NAME' removes all images with the name NAME
func (srv *Server) CmdRmi(stdin io.ReadCloser, stdout io.Writer, args ...string) (err error) {
	cmd := rcli.Subcmd(stdout, "rmimage", "[OPTIONS] IMAGE", "Remove an image")
	if cmd.Parse(args) != nil || cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	for _, name := range cmd.Args() {
		if err := srv.runtime.graph.Delete(name); err != nil {
			return err
		}
	}
	return nil
}

func (srv *Server) CmdRm(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "rm", "[OPTIONS] CONTAINER", "Remove a container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	for _, name := range cmd.Args() {
		container := srv.runtime.Get(name)
		if container == nil {
			return errors.New("No such container: " + name)
		}
		if err := srv.runtime.Destroy(container); err != nil {
			fmt.Fprintln(stdout, "Error destroying container "+name+": "+err.Error())
		}
	}
	return nil
}

// 'docker kill NAME' kills a running container
func (srv *Server) CmdKill(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "kill", "[OPTIONS] CONTAINER [CONTAINER...]", "Kill a running container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	for _, name := range cmd.Args() {
		container := srv.runtime.Get(name)
		if container == nil {
			return errors.New("No such container: " + name)
		}
		if err := container.Kill(); err != nil {
			fmt.Fprintln(stdout, "Error killing container "+name+": "+err.Error())
		}
	}
	return nil
}

func (srv *Server) CmdImport(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "import", "[OPTIONS] URL|- [REPOSITORY [TAG]]", "Create a new filesystem image from the contents of a tarball")
	var archive io.Reader
	var resp *http.Response

	if err := cmd.Parse(args); err != nil {
		return nil
	}
	src := cmd.Arg(0)
	if src == "" {
		return errors.New("Not enough arguments")
	} else if src == "-" {
		archive = stdin
	} else {
		u, err := url.Parse(src)
		if err != nil {
			return err
		}
		if u.Scheme == "" {
			u.Scheme = "http"
		}
		fmt.Fprintf(stdout, "Downloading from %s\n", u.String())
		// Download with curl (pretty progress bar)
		// If curl is not available, fallback to http.Get()
		resp, err = Download(u.String(), stdout)
		if err != nil {
			return err
		}
		archive = ProgressReader(resp.Body, int(resp.ContentLength), stdout)
	}
	img, err := srv.runtime.graph.Create(archive, "", "Imported from "+src)
	if err != nil {
		return err
	}
	// Optionally register the image at REPO/TAG
	if repository := cmd.Arg(1); repository != "" {
		tag := cmd.Arg(2) // Repository will handle an empty tag properly
		if err := srv.runtime.repositories.Set(repository, tag, img.Id); err != nil {
			return err
		}
	}
	fmt.Fprintln(stdout, img.Id)
	return nil
}

func (srv *Server) CmdImages(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "images", "[OPTIONS] [NAME]", "List images")
	//limit := cmd.Int("l", 0, "Only show the N most recent versions of each image")
	quiet := cmd.Bool("q", false, "only show numeric IDs")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() > 1 {
		cmd.Usage()
		return nil
	}
	var nameFilter string
	if cmd.NArg() == 1 {
		nameFilter = cmd.Arg(0)
	}
	w := tabwriter.NewWriter(stdout, 20, 1, 3, ' ', 0)
	if !*quiet {
		fmt.Fprintf(w, "REPOSITORY\tTAG\tID\tCREATED\tPARENT\n")
	}
	allImages, err := srv.runtime.graph.Map()
	if err != nil {
		return err
	}
	for name, repository := range srv.runtime.repositories.Repositories {
		if nameFilter != "" && name != nameFilter {
			continue
		}
		for tag, id := range repository {
			image, err := srv.runtime.graph.Get(id)
			if err != nil {
				log.Printf("Warning: couldn't load %s from %s/%s: %s", id, name, tag, err)
				continue
			}
			delete(allImages, id)
			if !*quiet {
				for idx, field := range []string{
					/* REPOSITORY */ name,
					/* TAG */ tag,
					/* ID */ id,
					/* CREATED */ HumanDuration(time.Now().Sub(image.Created)) + " ago",
					/* PARENT */ image.Parent,
				} {
					if idx == 0 {
						w.Write([]byte(field))
					} else {
						w.Write([]byte("\t" + field))
					}
				}
				w.Write([]byte{'\n'})
			} else {
				stdout.Write([]byte(image.Id + "\n"))
			}
		}
	}
	// Display images which aren't part of a 
	if nameFilter != "" {
		for id, image := range allImages {
			if !*quiet {
				for idx, field := range []string{
					/* REPOSITORY */ "",
					/* TAG */ "",
					/* ID */ id,
					/* CREATED */ HumanDuration(time.Now().Sub(image.Created)) + " ago",
					/* PARENT */ image.Parent,
				} {
					if idx == 0 {
						w.Write([]byte(field))
					} else {
						w.Write([]byte("\t" + field))
					}
				}
			} else {
				stdout.Write([]byte(image.Id + "\n"))
			}
		}
	}
	if !*quiet {
		w.Flush()
	}
	return nil
}

func (srv *Server) CmdPs(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout,
		"ps", "[OPTIONS]", "List containers")
	quiet := cmd.Bool("q", false, "Only display numeric IDs")
	fl_all := cmd.Bool("a", false, "Show all containers. Only running containers are shown by default.")
	fl_full := cmd.Bool("notrunc", false, "Don't truncate output")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	w := tabwriter.NewWriter(stdout, 12, 1, 3, ' ', 0)
	if !*quiet {
		fmt.Fprintf(w, "ID\tIMAGE\tCOMMAND\tCREATED\tSTATUS\tCOMMENT\n")
	}
	for _, container := range srv.runtime.List() {
		if !container.State.Running && !*fl_all {
			continue
		}
		if !*quiet {
			command := fmt.Sprintf("%s %s", container.Path, strings.Join(container.Args, " "))
			if !*fl_full {
				command = Trunc(command, 20)
			}
			for idx, field := range []string{
				/* ID */ container.Id,
				/* IMAGE */ container.Image,
				/* COMMAND */ command,
				/* CREATED */ HumanDuration(time.Now().Sub(container.Created)) + " ago",
				/* STATUS */ container.State.String(),
				/* COMMENT */ "",
			} {
				if idx == 0 {
					w.Write([]byte(field))
				} else {
					w.Write([]byte("\t" + field))
				}
			}
			w.Write([]byte{'\n'})
		} else {
			stdout.Write([]byte(container.Id + "\n"))
		}
	}
	if !*quiet {
		w.Flush()
	}
	return nil
}

func (srv *Server) CmdCommit(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout,
		"commit", "[OPTIONS] CONTAINER [DEST]",
		"Create a new image from a container's changes")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	containerName, imgName := cmd.Arg(0), cmd.Arg(1)
	if containerName == "" || imgName == "" {
		cmd.Usage()
		return nil
	}
	if container := srv.runtime.Get(containerName); container != nil {
		// FIXME: freeze the container before copying it to avoid data corruption?
		// FIXME: this shouldn't be in commands.
		rwTar, err := container.ExportRw()
		if err != nil {
			return err
		}
		// Create a new image from the container's base layers + a new layer from container changes
		img, err := srv.runtime.graph.Create(rwTar, container.Image, "")
		if err != nil {
			return err
		}

		fmt.Fprintln(stdout, img.Id)
		return nil
	}
	return errors.New("No such container: " + containerName)
}

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
	return errors.New("No such container: " + name)
}

func (srv *Server) CmdDiff(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout,
		"diff", "CONTAINER [OPTIONS]",
		"Inspect changes on a container's filesystem")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		return errors.New("Not enough arguments")
	}
	if container := srv.runtime.Get(cmd.Arg(0)); container == nil {
		return errors.New("No such container")
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

func (srv *Server) CmdLogs(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "logs", "[OPTIONS] CONTAINER", "Fetch the logs of a container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}
	name := cmd.Arg(0)
	if container := srv.runtime.Get(name); container != nil {
		log_stdout, err := container.ReadLog("stdout")
		if err != nil {
			return err
		}
		log_stderr, err := container.ReadLog("stderr")
		if err != nil {
			return err
		}
		// FIXME: Interpolate stdout and stderr instead of concatenating them
		// FIXME: Differentiate stdout and stderr in the remote protocol
		if _, err := io.Copy(stdout, log_stdout); err != nil {
			return err
		}
		if _, err := io.Copy(stdout, log_stderr); err != nil {
			return err
		}
		return nil
	}
	return errors.New("No such container: " + cmd.Arg(0))
}

func (srv *Server) CmdAttach(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "attach", "[OPTIONS]", "Attach to a running container")
	fl_i := cmd.Bool("i", false, "Attach to stdin")
	fl_o := cmd.Bool("o", true, "Attach to stdout")
	fl_e := cmd.Bool("e", true, "Attach to stderr")
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
		return errors.New("No such container: " + name)
	}
	var wg sync.WaitGroup
	if *fl_i {
		c_stdin, err := container.StdinPipe()
		if err != nil {
			return err
		}
		wg.Add(1)
		go func() { io.Copy(c_stdin, stdin); wg.Add(-1) }()
	}
	if *fl_o {
		c_stdout, err := container.StdoutPipe()
		if err != nil {
			return err
		}
		wg.Add(1)
		go func() { io.Copy(stdout, c_stdout); wg.Add(-1) }()
	}
	if *fl_e {
		c_stderr, err := container.StderrPipe()
		if err != nil {
			return err
		}
		wg.Add(1)
		go func() { io.Copy(stdout, c_stderr); wg.Add(-1) }()
	}
	wg.Wait()
	return nil
}

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

func (srv *Server) CmdRun(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "run", "[OPTIONS] IMAGE COMMAND [ARG...]", "Run a command in a new container")
	fl_user := cmd.String("u", "", "Username or UID")
	fl_attach := cmd.Bool("a", false, "Attach stdin and stdout")
	fl_stdin := cmd.Bool("i", false, "Keep stdin open even if not attached")
	fl_tty := cmd.Bool("t", false, "Allocate a pseudo-tty")
	fl_memory := cmd.Int64("m", 0, "Memory limit (in bytes)")
	var fl_ports ports

	cmd.Var(&fl_ports, "p", "Map a network port to the container")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	name := cmd.Arg(0)
	var cmdline []string

	if len(cmd.Args()) >= 2 {
		cmdline = cmd.Args()[1:]
	}
	// Choose a default image if needed
	if name == "" {
		name = "base"
	}

	// Choose a default command if needed
	if len(cmdline) == 0 {
		*fl_stdin = true
		*fl_tty = true
		*fl_attach = true
		cmdline = []string{"/bin/bash", "-i"}
	}

	// Create new container
	container, err := srv.runtime.Create(cmdline[0], cmdline[1:], name,
		&Config{
			Ports:     fl_ports,
			User:      *fl_user,
			Tty:       *fl_tty,
			OpenStdin: *fl_stdin,
			Memory:    *fl_memory,
		})
	if err != nil {
		return errors.New("Error creating container: " + err.Error())
	}
	if *fl_stdin {
		cmd_stdin, err := container.StdinPipe()
		if err != nil {
			return err
		}
		if *fl_attach {
			Go(func() error {
				_, err := io.Copy(cmd_stdin, stdin)
				cmd_stdin.Close()
				return err
			})
		}
	}
	// Run the container
	if *fl_attach {
		cmd_stderr, err := container.StderrPipe()
		if err != nil {
			return err
		}
		cmd_stdout, err := container.StdoutPipe()
		if err != nil {
			return err
		}
		if err := container.Start(); err != nil {
			return err
		}
		sending_stdout := Go(func() error {
			_, err := io.Copy(stdout, cmd_stdout)
			return err
		})
		sending_stderr := Go(func() error {
			_, err := io.Copy(stdout, cmd_stderr)
			return err
		})
		err_sending_stdout := <-sending_stdout
		err_sending_stderr := <-sending_stderr
		if err_sending_stdout != nil {
			return err_sending_stdout
		}
		if err_sending_stderr != nil {
			return err_sending_stderr
		}
		container.Wait()
	} else {
		if err := container.Start(); err != nil {
			return err
		}
		fmt.Fprintln(stdout, container.Id)
	}
	return nil
}

func NewServer() (*Server, error) {
	rand.Seed(time.Now().UTC().UnixNano())
	if runtime.GOARCH != "amd64" {
		log.Fatalf("The docker runtime currently only supports amd64 (not %s). This will change in the future. Aborting.", runtime.GOARCH)
	}
	// if err != nil {
	// 	return nil, err
	// }
	runtime, err := New()
	if err != nil {
		return nil, err
	}
	srv := &Server{
		runtime: runtime,
	}
	return srv, nil
}

type Server struct {
	runtime *Runtime
}
