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

const VERSION = "0.1.0"

func (srv *Server) Name() string {
	return "docker"
}

// FIXME: Stop violating DRY by repeating usage here and in Subcmd declarations
func (srv *Server) Help() string {
	help := "Usage: docker COMMAND [arg...]\n\nA self-sufficient runtime for linux containers.\n\nCommands:\n"
	for _, cmd := range [][]interface{}{
		{"attach", "Attach to a running container"},
		{"commit", "Create a new image from a container's changes"},
		{"diff", "Inspect changes on a container's filesystem"},
		{"export", "Stream the contents of a container as a tar archive"},
		{"history", "Show the history of an image"},
		{"images", "List images"},
		{"import", "Create a new filesystem image from the contents of a tarball"},
		{"info", "Display system-wide information"},
		{"inspect", "Return low-level information on a container"},
		{"kill", "Kill a running container"},
		{"login", "Register or Login to the docker registry server"},
		{"logs", "Fetch the logs of a container"},
		{"port", "Lookup the public-facing port which is NAT-ed to PRIVATE_PORT"},
		{"ps", "List containers"},
		{"pull", "Pull an image or a repository to the docker registry server"},
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

	fmt.Fprint(stdout, "Username (", srv.runtime.authConfig.Username, "): ")
	fmt.Fscanf(stdin, "%s", &username)
	if username == "" {
		username = srv.runtime.authConfig.Username
	}
	if username != srv.runtime.authConfig.Username {
		fmt.Fprint(stdout, "Password: ")
		fmt.Fscanf(stdin, "%s", &password)

		if password == "" {
			return errors.New("Error : Password Required\n")
		}

		fmt.Fprint(stdout, "Email (", srv.runtime.authConfig.Email, "): ")
		fmt.Fscanf(stdin, "%s", &email)
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
		fmt.Fprintf(stdout, "Error : %s\n", err)
	} else {
		srv.runtime.authConfig = newAuthConfig
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

func (srv *Server) CmdHistory(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "history", "[OPTIONS] IMAGE", "Show the history of an image")
	if cmd.Parse(args) != nil || cmd.NArg() != 1 {
		cmd.Usage()
		return nil
	}
	image, err := srv.runtime.repositories.LookupImage(cmd.Arg(0))
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(stdout, 20, 1, 3, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "ID\tCREATED\tCREATED BY\n")
	return image.WalkHistory(func(img *Image) error {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			srv.runtime.repositories.ImageName(img.Id),
			HumanDuration(time.Now().Sub(img.Created))+" ago",
			strings.Join(img.ContainerConfig.Cmd, " "),
		)
		return nil
	})
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
			u.Host = src
			u.Path = ""
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
	fmt.Fprintln(stdout, img.Id)
	return nil
}

func (srv *Server) CmdPush(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
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
		} else {
			return err
		}
		return nil
	}
	err = srv.runtime.graph.PushImage(stdout, img, srv.runtime.authConfig)
	if err != nil {
		return err
	}
	return nil
}

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

	if srv.runtime.graph.LookupRemoteImage(remote, srv.runtime.authConfig) {
		fmt.Fprintf(stdout, "Pulling %s...\n", remote)
		if err := srv.runtime.graph.PullImage(remote, srv.runtime.authConfig); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Pulled\n")
		return nil
	}
	// FIXME: Allow pull repo:tag
	fmt.Fprintf(stdout, "Pulling %s...\n", remote)
	if err := srv.runtime.graph.PullRepository(stdout, remote, "", srv.runtime.repositories, srv.runtime.authConfig); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Pull completed\n")
	return nil
}

func (srv *Server) CmdImages(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "images", "[OPTIONS] [NAME]", "List images")
	//limit := cmd.Int("l", 0, "Only show the N most recent versions of each image")
	quiet := cmd.Bool("q", false, "only show numeric IDs")
	fl_a := cmd.Bool("a", false, "show all images")
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
	var allImages map[string]*Image
	var err error
	if *fl_a {
		allImages, err = srv.runtime.graph.Map()
	} else {
		allImages, err = srv.runtime.graph.Heads()
	}
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
					/* PARENT */ srv.runtime.repositories.ImageName(image.Parent),
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
	if nameFilter == "" {
		for id, image := range allImages {
			if !*quiet {
				for idx, field := range []string{
					/* REPOSITORY */ "<none>",
					/* TAG */ "<none>",
					/* ID */ id,
					/* CREATED */ HumanDuration(time.Now().Sub(image.Created)) + " ago",
					/* PARENT */ srv.runtime.repositories.ImageName(image.Parent),
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
				/* IMAGE */ srv.runtime.repositories.ImageName(container.Image),
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
		"commit", "[OPTIONS] CONTAINER [REPOSITORY [TAG]]",
		"Create a new image from a container's changes")
	fl_comment := cmd.String("m", "", "Commit message")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	containerName, repository, tag := cmd.Arg(0), cmd.Arg(1), cmd.Arg(2)
	if containerName == "" {
		cmd.Usage()
		return nil
	}
	img, err := srv.runtime.Commit(containerName, repository, tag, *fl_comment)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, img.Id)
	return nil
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

// ListOpts type
type ListOpts []string

func (opts *ListOpts) String() string {
	return fmt.Sprint(*opts)
}

func (opts *ListOpts) Set(value string) error {
	*opts = append(*opts, value)
	return nil
}

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

func (srv *Server) CmdRun(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	config, err := ParseRun(args)
	if err != nil {
		return err
	}
	if config.Image == "" {
		return fmt.Errorf("Image not specified")
	}
	if len(config.Cmd) == 0 {
		return fmt.Errorf("Command not specified")
	}
	// Create new container
	container, err := srv.runtime.Create(config)
	if err != nil {
		return errors.New("Error creating container: " + err.Error())
	}
	if config.OpenStdin {
		cmd_stdin, err := container.StdinPipe()
		if err != nil {
			return err
		}
		if !config.Detach {
			Go(func() error {
				_, err := io.Copy(cmd_stdin, stdin)
				cmd_stdin.Close()
				return err
			})
		}
	}
	// Run the container
	if !config.Detach {
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
	runtime, err := NewRuntime()
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
