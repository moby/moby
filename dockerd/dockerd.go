package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/future"
	"github.com/dotcloud/docker/image"
	"github.com/dotcloud/docker/rcli"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
)

const VERSION = "0.0.1"

func (srv *Server) Name() string {
	return "docker"
}

func (srv *Server) Help() string {
	help := "Usage: docker COMMAND [arg...]\n\nA self-sufficient runtime for linux containers.\n\nCommands:\n"
	for _, cmd := range [][]interface{}{
		{"run", "Run a command in a container"},
		{"ps", "Display a list of containers"},
		{"pull", "Download a tarball and create a container from it"},
		{"put", "Upload a tarball and create a container from it"},
		{"rm", "Remove containers"},
		{"kill", "Kill a running container"},
		{"wait", "Wait for the state of a container to change"},
		{"stop", "Stop a running container"},
		{"logs", "Fetch the logs of a container"},
		{"diff", "Inspect changes on a container's filesystem"},
		{"commit", "Save the state of a container"},
		{"attach", "Attach to the standard inputs and outputs of a running container"},
		{"info", "Display system-wide information"},
		{"tar", "Stream the contents of a container as a tar archive"},
		{"web", "Generate a web UI"},
		{"images", "List images"},
	} {
		help += fmt.Sprintf("    %-10.10s%s\n", cmd...)
	}
	return help
}

// 'docker info': display system-wide information.
func (srv *Server) CmdInfo(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	fmt.Fprintf(stdout, "containers: %d\nversion: %s\nimages: %d\n",
		len(srv.containers.List()),
		VERSION,
		len(srv.images.ById))
	return nil
}

func (srv *Server) CmdStop(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "stop", "[OPTIONS] NAME", "Stop a running container")
	if err := cmd.Parse(args); err != nil {
		cmd.Usage()
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	for _, name := range cmd.Args() {
		if container := srv.containers.Get(name); container != nil {
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

func (srv *Server) CmdUmount(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "umount", "[OPTIONS] NAME", "umount a container's filesystem (debug only)")
	if err := cmd.Parse(args); err != nil {
		cmd.Usage()
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	for _, name := range cmd.Args() {
		if container := srv.containers.Get(name); container != nil {
			if err := container.Filesystem.Umount(); err != nil {
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
		cmd.Usage()
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	for _, name := range cmd.Args() {
		if container := srv.containers.Get(name); container != nil {
			if err := container.Filesystem.Mount(); err != nil {
				return err
			}
			fmt.Fprintln(stdout, container.Id)
		} else {
			return errors.New("No such container: " + name)
		}
	}
	return nil
}

func (srv *Server) CmdCat(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "cat", "[OPTIONS] CONTAINER PATH", "write the contents of a container's file to standard output")
	if err := cmd.Parse(args); err != nil {
		cmd.Usage()
		return nil
	}
	if cmd.NArg() < 2 {
		cmd.Usage()
		return nil
	}
	name, path := cmd.Arg(0), cmd.Arg(1)
	if container := srv.containers.Get(name); container != nil {
		if f, err := container.Filesystem.OpenFile(path, os.O_RDONLY, 0); err != nil {
			return err
		} else if _, err := io.Copy(stdout, f); err != nil {
			return err
		}
		return nil
	}
	return errors.New("No such container: " + name)
}

func (srv *Server) CmdWrite(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "write", "[OPTIONS] CONTAINER PATH", "write the contents of standard input to a container's file")
	if err := cmd.Parse(args); err != nil {
		cmd.Usage()
		return nil
	}
	if cmd.NArg() < 2 {
		cmd.Usage()
		return nil
	}
	name, path := cmd.Arg(0), cmd.Arg(1)
	if container := srv.containers.Get(name); container != nil {
		if f, err := container.Filesystem.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0600); err != nil {
			return err
		} else if _, err := io.Copy(f, stdin); err != nil {
			return err
		}
		return nil
	}
	return errors.New("No such container: " + name)
}

func (srv *Server) CmdLs(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "ls", "[OPTIONS] CONTAINER PATH", "List the contents of a container's directory")
	if err := cmd.Parse(args); err != nil {
		cmd.Usage()
		return nil
	}
	if cmd.NArg() < 2 {
		cmd.Usage()
		return nil
	}
	name, path := cmd.Arg(0), cmd.Arg(1)
	if container := srv.containers.Get(name); container != nil {
		if files, err := container.Filesystem.ReadDir(path); err != nil {
			return err
		} else {
			for _, f := range files {
				fmt.Fprintln(stdout, f.Name())
			}
		}
		return nil
	}
	return errors.New("No such container: " + name)
}

func (srv *Server) CmdInspect(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "inspect", "[OPTIONS] CONTAINER", "Return low-level information on a container")
	if err := cmd.Parse(args); err != nil {
		cmd.Usage()
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	name := cmd.Arg(0)
	var obj interface{}
	if container := srv.containers.Get(name); container != nil {
		obj = container
	} else if image := srv.images.Find(name); image != nil {
		obj = image
	} else {
		return errors.New("No such container or image: " + name)
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
	return nil
}

// 'docker rmi NAME' removes all images with the name NAME
func (srv *Server) CmdRmi(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "rmimage", "[OPTIONS] IMAGE", "Remove an image")
	if err := cmd.Parse(args); err != nil {
		cmd.Usage()
		return nil
	}
	if cmd.NArg() < 1 {
		cmd.Usage()
		return nil
	}
	for _, name := range cmd.Args() {
		image := srv.images.Find(name)
		if image == nil {
			return errors.New("No such image: " + name)
		}
		if err := srv.images.Delete(name); err != nil {
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
		container := srv.containers.Get(name)
		if container == nil {
			return errors.New("No such container: " + name)
		}
		if err := srv.containers.Destroy(container); err != nil {
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
		container := srv.containers.Get(name)
		if container == nil {
			return errors.New("No such container: " + name)
		}
		if err := container.Kill(); err != nil {
			fmt.Fprintln(stdout, "Error killing container "+name+": "+err.Error())
		}
	}
	return nil
}

func (srv *Server) CmdPull(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "pull", "[OPTIONS] NAME", "Download a new image from a remote location")
	fl_bzip2 := cmd.Bool("j", false, "Bzip2 compression")
	fl_gzip := cmd.Bool("z", false, "Gzip compression")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	var compression image.Compression
	if *fl_bzip2 {
		compression = image.Bzip2
	} else if *fl_gzip {
		compression = image.Gzip
	}
	name := cmd.Arg(0)
	if name == "" {
		return errors.New("Not enough arguments")
	}
	u, err := url.Parse(name)
	if err != nil {
		return err
	}
	if u.Scheme == "" {
		u.Scheme = "http"
	}
	// FIXME: hardcode a mirror URL that does not depend on a single provider.
	if u.Host == "" {
		u.Host = "s3.amazonaws.com"
		u.Path = path.Join("/docker.io/images", u.Path)
	}
	fmt.Fprintf(stdout, "Downloading %s from %s...\n", name, u.String())
	resp, err := http.Get(u.String())
	if err != nil {
		return err
	}
	img, err := srv.images.Import(name, resp.Body, stdout, nil, compression)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, img.Id)
	return nil
}

func (srv *Server) CmdPut(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "put", "[OPTIONS] NAME", "Import a new image from a local archive.")
	fl_bzip2 := cmd.Bool("j", false, "Bzip2 compression")
	fl_gzip := cmd.Bool("z", false, "Gzip compression")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	var compression image.Compression
	if *fl_bzip2 {
		compression = image.Bzip2
	} else if *fl_gzip {
		compression = image.Gzip
	}
	name := cmd.Arg(0)
	if name == "" {
		return errors.New("Not enough arguments")
	}
	img, err := srv.images.Import(name, stdin, stdout, nil, compression)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, img.Id)
	return nil
}

func (srv *Server) CmdImages(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "images", "[OPTIONS] [NAME]", "List images")
	limit := cmd.Int("l", 0, "Only show the N most recent versions of each image")
	quiet := cmd.Bool("q", false, "only show numeric IDs")
	cmd.Parse(args)
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
		fmt.Fprintf(w, "NAME\tID\tCREATED\tPARENT\n")
	}
	for _, name := range srv.images.Names() {
		if nameFilter != "" && nameFilter != name {
			continue
		}
		for idx, img := range *srv.images.ByName[name] {
			if *limit > 0 && idx >= *limit {
				break
			}
			if !*quiet {
				id := img.Id
				if !img.IdIsFinal() {
					id += "..."
				}
				for idx, field := range []string{
					/* NAME */ name,
					/* ID */ id,
					/* CREATED */ future.HumanDuration(time.Now().Sub(img.Created)) + " ago",
					/* PARENT */ img.Parent,
				} {
					if idx == 0 {
						w.Write([]byte(field))
					} else {
						w.Write([]byte("\t" + field))
					}
				}
				w.Write([]byte{'\n'})
			} else {
				stdout.Write([]byte(img.Id + "\n"))
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
	for _, container := range srv.containers.List() {
		comment := container.GetUserData("comment")
		if !container.State.Running && !*fl_all {
			continue
		}
		if !*quiet {
			command := fmt.Sprintf("%s %s", container.Path, strings.Join(container.Args, " "))
			if !*fl_full {
				command = docker.Trunc(command, 20)
			}
			for idx, field := range []string{
				/* ID */ container.Id,
				/* IMAGE */ container.GetUserData("image"),
				/* COMMAND */ command,
				/* CREATED */ future.HumanDuration(time.Now().Sub(container.Created)) + " ago",
				/* STATUS */ container.State.String(),
				/* COMMENT */ comment,
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

func (srv *Server) CmdLayers(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout,
		"layers", "[OPTIONS]",
		"List filesystem layers (debug only)")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	for _, layer := range srv.images.Layers.List() {
		fmt.Fprintln(stdout, layer)
	}
	return nil
}

func (srv *Server) CmdCp(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout,
		"cp", "[OPTIONS] IMAGE NAME",
		"Create a copy of IMAGE and call it NAME")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if newImage, err := srv.images.Copy(cmd.Arg(0), cmd.Arg(1)); err != nil {
		return err
	} else {
		fmt.Fprintln(stdout, newImage.Id)
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
	if container := srv.containers.Get(containerName); container != nil {
		// FIXME: freeze the container before copying it to avoid data corruption?
		rwTar, err := docker.Tar(container.Filesystem.RWPath)
		if err != nil {
			return err
		}
		// Create a new image from the container's base layers + a new layer from container changes
		parentImg := srv.images.Find(container.GetUserData("image"))
		img, err := srv.images.Import(imgName, rwTar, stdout, parentImg, image.Uncompressed)
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, img.Id)
		return nil
	}
	return errors.New("No such container: " + containerName)
}

func (srv *Server) CmdTar(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout,
		"tar", "CONTAINER",
		"Stream the contents of a container as a tar archive")
	fl_sparse := cmd.Bool("s", false, "Generate a sparse tar stream (top layer + reference to bottom layers)")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if *fl_sparse {
		return errors.New("Sparse mode not yet implemented") // FIXME
	}
	name := cmd.Arg(0)
	if container := srv.containers.Get(name); container != nil {
		data, err := container.Filesystem.Tar()
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
	if container := srv.containers.Get(cmd.Arg(0)); container == nil {
		return errors.New("No such container")
	} else {
		changes, err := container.Filesystem.Changes()
		if err != nil {
			return err
		}
		for _, change := range changes {
			fmt.Fprintln(stdout, change.String())
		}
	}
	return nil
}

func (srv *Server) CmdReset(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout,
		"reset", "CONTAINER [OPTIONS]",
		"Reset changes to a container's filesystem")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if cmd.NArg() < 1 {
		return errors.New("Not enough arguments")
	}
	for _, name := range cmd.Args() {
		if container := srv.containers.Get(name); container != nil {
			if err := container.Filesystem.Reset(); err != nil {
				return errors.New("Reset " + container.Id + ": " + err.Error())
			}
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
	if container := srv.containers.Get(name); container != nil {
		if _, err := io.Copy(stdout, container.StdoutLog()); err != nil {
			return err
		}
		if _, err := io.Copy(stdout, container.StderrLog()); err != nil {
			return err
		}
		return nil
	}
	return errors.New("No such container: " + cmd.Arg(0))
}

func (srv *Server) CreateContainer(img *image.Image, user string, tty bool, openStdin bool, comment string, cmd string, args ...string) (*docker.Container, error) {
	id := future.RandomId()[:8]
	container, err := srv.containers.Create(id, cmd, args, img.Layers,
		&docker.Config{Hostname: id, User: user, Tty: tty, OpenStdin: openStdin})
	if err != nil {
		return nil, err
	}
	if err := container.SetUserData("image", img.Id); err != nil {
		srv.containers.Destroy(container)
		return nil, errors.New("Error setting container userdata: " + err.Error())
	}
	if err := container.SetUserData("comment", comment); err != nil {
		srv.containers.Destroy(container)
		return nil, errors.New("Error setting container userdata: " + err.Error())
	}
	return container, nil
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
	container := srv.containers.Get(name)
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

func (srv *Server) CmdRun(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "run", "[OPTIONS] IMAGE COMMAND [ARG...]", "Run a command in a new container")
	fl_attach := cmd.Bool("a", false, "Attach stdin and stdout")
	fl_stdin := cmd.Bool("i", false, "Keep stdin open even if not attached")
	fl_tty := cmd.Bool("t", false, "Allocate a pseudo-tty")
	fl_user := cmd.String("u", "0", "Username or UID")
	fl_comment := cmd.String("c", "", "Comment")
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
	// Find the image
	img := srv.images.Find(name)
	if img == nil {
		return errors.New("No such image: " + name)
	}
	// Create new container
	container, err := srv.CreateContainer(img, *fl_user, *fl_tty, *fl_stdin, *fl_comment, cmdline[0], cmdline[1:]...)
	if err != nil {
		return errors.New("Error creating container: " + err.Error())
	}
	if *fl_stdin {
		cmd_stdin, err := container.StdinPipe()
		if err != nil {
			return err
		}
		if *fl_attach {
			future.Go(func() error {
				log.Printf("CmdRun(): start receiving stdin\n")
				_, err := io.Copy(cmd_stdin, stdin)
				log.Printf("CmdRun(): done receiving stdin\n")
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
		sending_stdout := future.Go(func() error {
			_, err := io.Copy(stdout, cmd_stdout)
			return err
		})
		sending_stderr := future.Go(func() error {
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

func main() {
	if docker.SelfPath() == "/sbin/init" {
		// Running in init mode
		docker.SysInit()
		return
	}
	future.Seed()
	flag.Parse()
	d, err := New()
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		if err := rcli.ListenAndServeHTTP("127.0.0.1:8080", d); err != nil {
			log.Fatal(err)
		}
	}()
	// FIXME: we want to use unix sockets here, but net.UnixConn doesn't expose
	// CloseWrite(), which we need to cleanly signal that stdin is closed without
	// closing the connection.
	// See http://code.google.com/p/go/issues/detail?id=3345
	if err := rcli.ListenAndServe("tcp", "127.0.0.1:4242", d); err != nil {
		log.Fatal(err)
	}
}

func New() (*Server, error) {
	images, err := image.New("/var/lib/docker/images")
	if err != nil {
		return nil, err
	}
	containers, err := docker.New()
	if err != nil {
		return nil, err
	}
	srv := &Server{
		images:     images,
		containers: containers,
	}
	return srv, nil
}

func (srv *Server) CmdMirror(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	_, err := io.Copy(stdout, stdin)
	return err
}

func (srv *Server) CmdDebug(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	for {
		if line, err := bufio.NewReader(stdin).ReadString('\n'); err == nil {
			fmt.Printf("--- %s", line)
		} else if err == io.EOF {
			if len(line) > 0 {
				fmt.Printf("--- %s\n", line)
			}
			break
		} else {
			return err
		}
	}
	return nil
}

func (srv *Server) CmdWeb(stdin io.ReadCloser, stdout io.Writer, args ...string) error {
	cmd := rcli.Subcmd(stdout, "web", "[OPTIONS]", "A web UI for docker")
	showurl := cmd.Bool("u", false, "Return the URL of the web UI")
	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if *showurl {
		fmt.Fprintln(stdout, "http://localhost:4242/web")
	} else {
		if file, err := os.Open("dockerweb.html"); err != nil {
			return err
		} else if _, err := io.Copy(stdout, file); err != nil {
			return err
		}
	}
	return nil
}

type Server struct {
	containers *docker.Docker
	images     *image.Store
}
