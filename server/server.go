// DEPRECATION NOTICE. PLEASE DO NOT ADD ANYTHING TO THIS FILE.
//
// server/server.go is deprecated. We are working on breaking it up into smaller, cleaner
// pieces which will be easier to find and test. This will help make the code less
// redundant and more readable.
//
// Contributors, please don't add anything to server/server.go, unless it has the explicit
// goal of helping the deprecation effort.
//
// Maintainers, please refuse patches which add code to server/server.go.
//
// Instead try the following files:
// * For code related to local image management, try graph/
// * For code related to image downloading, uploading, remote search etc, try registry/
// * For code related to the docker daemon, try daemon/
// * For small utilities which could potentially be useful outside of Docker, try pkg/
// * For miscalleneous "util" functions which are docker-specific, try encapsulating them
//     inside one of the subsystems above. If you really think they should be more widely
//     available, are you sure you can't remove the docker dependencies and move them to
//     pkg? In last resort, you can add them to utils/ (but please try not to).

package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	gosignal "os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/daemon"
	"github.com/dotcloud/docker/daemonconfig"
	"github.com/dotcloud/docker/dockerversion"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/graph"
	"github.com/dotcloud/docker/image"
	"github.com/dotcloud/docker/pkg/graphdb"
	"github.com/dotcloud/docker/pkg/signal"
	"github.com/dotcloud/docker/registry"
	"github.com/dotcloud/docker/runconfig"
	"github.com/dotcloud/docker/utils"
	"github.com/dotcloud/docker/utils/filters"
)

func (srv *Server) handlerWrap(h engine.Handler) engine.Handler {
	return func(job *engine.Job) engine.Status {
		if !srv.IsRunning() {
			return job.Errorf("Server is not running")
		}
		srv.tasks.Add(1)
		defer srv.tasks.Done()
		return h(job)
	}
}

// jobInitApi runs the remote api server `srv` as a daemon,
// Only one api server can run at the same time - this is enforced by a pidfile.
// The signals SIGINT, SIGQUIT and SIGTERM are intercepted for cleanup.
func InitServer(job *engine.Job) engine.Status {
	job.Logf("Creating server")
	srv, err := NewServer(job.Eng, daemonconfig.ConfigFromJob(job))
	if err != nil {
		return job.Error(err)
	}
	if srv.daemon.Config().Pidfile != "" {
		job.Logf("Creating pidfile")
		if err := utils.CreatePidFile(srv.daemon.Config().Pidfile); err != nil {
			// FIXME: do we need fatal here instead of returning a job error?
			log.Fatal(err)
		}
	}
	job.Logf("Setting up signal traps")
	c := make(chan os.Signal, 1)
	gosignal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		interruptCount := uint32(0)
		for sig := range c {
			go func(sig os.Signal) {
				log.Printf("Received signal '%v', starting shutdown of docker...\n", sig)
				switch sig {
				case os.Interrupt, syscall.SIGTERM:
					// If the user really wants to interrupt, let him do so.
					if atomic.LoadUint32(&interruptCount) < 3 {
						atomic.AddUint32(&interruptCount, 1)
						// Initiate the cleanup only once
						if atomic.LoadUint32(&interruptCount) == 1 {
							utils.RemovePidFile(srv.daemon.Config().Pidfile)
							srv.Close()
						} else {
							return
						}
					} else {
						log.Printf("Force shutdown of docker, interrupting cleanup\n")
					}
				case syscall.SIGQUIT:
				}
				os.Exit(128 + int(sig.(syscall.Signal)))
			}(sig)
		}
	}()
	job.Eng.Hack_SetGlobalVar("httpapi.server", srv)
	job.Eng.Hack_SetGlobalVar("httpapi.daemon", srv.daemon)

	// FIXME: 'insert' is deprecated and should be removed in a future version.
	for name, handler := range map[string]engine.Handler{
		"export":           srv.ContainerExport,
		"create":           srv.ContainerCreate,
		"stop":             srv.ContainerStop,
		"restart":          srv.ContainerRestart,
		"start":            srv.ContainerStart,
		"kill":             srv.ContainerKill,
		"pause":            srv.ContainerPause,
		"unpause":          srv.ContainerUnpause,
		"wait":             srv.ContainerWait,
		"tag":              srv.ImageTag, // FIXME merge with "image_tag"
		"resize":           srv.ContainerResize,
		"commit":           srv.ContainerCommit,
		"info":             srv.DockerInfo,
		"container_delete": srv.ContainerDestroy,
		"image_export":     srv.ImageExport,
		"images":           srv.Images,
		"history":          srv.ImageHistory,
		"viz":              srv.ImagesViz,
		"container_copy":   srv.ContainerCopy,
		"attach":           srv.ContainerAttach,
		"logs":             srv.ContainerLogs,
		"changes":          srv.ContainerChanges,
		"top":              srv.ContainerTop,
		"load":             srv.ImageLoad,
		"build":            srv.Build,
		"pull":             srv.ImagePull,
		"import":           srv.ImageImport,
		"image_delete":     srv.ImageDelete,
		"events":           srv.Events,
		"push":             srv.ImagePush,
		"containers":       srv.Containers,
	} {
		if err := job.Eng.Register(name, srv.handlerWrap(handler)); err != nil {
			return job.Error(err)
		}
	}
	// Install image-related commands from the image subsystem.
	// See `graph/service.go`
	if err := srv.daemon.Repositories().Install(job.Eng); err != nil {
		return job.Error(err)
	}
	// Install daemon-related commands from the daemon subsystem.
	// See `daemon/`
	if err := srv.daemon.Install(job.Eng); err != nil {
		return job.Error(err)
	}
	srv.SetRunning(true)
	return engine.StatusOK
}

func (srv *Server) ContainerPause(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER", job.Name)
	}
	name := job.Args[0]
	container := srv.daemon.Get(name)
	if container == nil {
		return job.Errorf("No such container: %s", name)
	}
	if err := container.Pause(); err != nil {
		return job.Errorf("Cannot pause container %s: %s", name, err)
	}
	return engine.StatusOK
}

func (srv *Server) ContainerUnpause(job *engine.Job) engine.Status {
	if n := len(job.Args); n < 1 || n > 2 {
		return job.Errorf("Usage: %s CONTAINER", job.Name)
	}
	name := job.Args[0]
	container := srv.daemon.Get(name)
	if container == nil {
		return job.Errorf("No such container: %s", name)
	}
	if err := container.Unpause(); err != nil {
		return job.Errorf("Cannot unpause container %s: %s", name, err)
	}
	return engine.StatusOK
}

// ContainerKill send signal to the container
// If no signal is given (sig 0), then Kill with SIGKILL and wait
// for the container to exit.
// If a signal is given, then just send it to the container and return.
func (srv *Server) ContainerKill(job *engine.Job) engine.Status {
	if n := len(job.Args); n < 1 || n > 2 {
		return job.Errorf("Usage: %s CONTAINER [SIGNAL]", job.Name)
	}
	var (
		name = job.Args[0]
		sig  uint64
		err  error
	)

	// If we have a signal, look at it. Otherwise, do nothing
	if len(job.Args) == 2 && job.Args[1] != "" {
		// Check if we passed the signal as a number:
		// The largest legal signal is 31, so let's parse on 5 bits
		sig, err = strconv.ParseUint(job.Args[1], 10, 5)
		if err != nil {
			// The signal is not a number, treat it as a string (either like "KILL" or like "SIGKILL")
			sig = uint64(signal.SignalMap[strings.TrimPrefix(job.Args[1], "SIG")])
			if sig == 0 {
				return job.Errorf("Invalid signal: %s", job.Args[1])
			}

		}
	}

	if container := srv.daemon.Get(name); container != nil {
		// If no signal is passed, or SIGKILL, perform regular Kill (SIGKILL + wait())
		if sig == 0 || syscall.Signal(sig) == syscall.SIGKILL {
			if err := container.Kill(); err != nil {
				return job.Errorf("Cannot kill container %s: %s", name, err)
			}
			srv.LogEvent("kill", container.ID, srv.daemon.Repositories().ImageName(container.Image))
		} else {
			// Otherwise, just send the requested signal
			if err := container.KillSig(int(sig)); err != nil {
				return job.Errorf("Cannot kill container %s: %s", name, err)
			}
			// FIXME: Add event for signals
		}
	} else {
		return job.Errorf("No such container: %s", name)
	}
	return engine.StatusOK
}

func (srv *Server) EvictListener(from int64) {
	srv.Lock()
	if old, ok := srv.listeners[from]; ok {
		delete(srv.listeners, from)
		close(old)
	}
	srv.Unlock()
}

func (srv *Server) Events(job *engine.Job) engine.Status {
	if len(job.Args) != 0 {
		return job.Errorf("Usage: %s", job.Name)
	}

	var (
		from    = time.Now().UTC().UnixNano()
		since   = job.GetenvInt64("since")
		until   = job.GetenvInt64("until")
		timeout = time.NewTimer(time.Unix(until, 0).Sub(time.Now()))
	)
	sendEvent := func(event *utils.JSONMessage) error {
		b, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("JSON error")
		}
		_, err = job.Stdout.Write(b)
		return err
	}

	listener := make(chan utils.JSONMessage)
	srv.Lock()
	if old, ok := srv.listeners[from]; ok {
		delete(srv.listeners, from)
		close(old)
	}
	srv.listeners[from] = listener
	srv.Unlock()
	job.Stdout.Write(nil) // flush
	if since != 0 {
		// If since, send previous events that happened after the timestamp and until timestamp
		for _, event := range srv.GetEvents() {
			if event.Time >= since && (event.Time <= until || until == 0) {
				err := sendEvent(&event)
				if err != nil && err.Error() == "JSON error" {
					continue
				}
				if err != nil {
					// On error, evict the listener
					srv.EvictListener(from)
					return job.Error(err)
				}
			}
		}
	}

	// If no until, disable timeout
	if until == 0 {
		timeout.Stop()
	}
	for {
		select {
		case event, ok := <-listener:
			if !ok { // Channel is closed: listener was evicted
				return engine.StatusOK
			}
			err := sendEvent(&event)
			if err != nil && err.Error() == "JSON error" {
				continue
			}
			if err != nil {
				// On error, evict the listener
				srv.EvictListener(from)
				return job.Error(err)
			}
		case <-timeout.C:
			return engine.StatusOK
		}
	}
	return engine.StatusOK
}

func (srv *Server) ContainerExport(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s container_id", job.Name)
	}
	name := job.Args[0]
	if container := srv.daemon.Get(name); container != nil {
		data, err := container.Export()
		if err != nil {
			return job.Errorf("%s: %s", name, err)
		}
		defer data.Close()

		// Stream the entire contents of the container (basically a volatile snapshot)
		if _, err := io.Copy(job.Stdout, data); err != nil {
			return job.Errorf("%s: %s", name, err)
		}
		// FIXME: factor job-specific LogEvent to engine.Job.Run()
		srv.LogEvent("export", container.ID, srv.daemon.Repositories().ImageName(container.Image))
		return engine.StatusOK
	}
	return job.Errorf("No such container: %s", name)
}

// ImageExport exports all images with the given tag. All versions
// containing the same tag are exported. The resulting output is an
// uncompressed tar ball.
// name is the set of tags to export.
// out is the writer where the images are written to.
func (srv *Server) ImageExport(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s IMAGE\n", job.Name)
	}
	name := job.Args[0]
	// get image json
	tempdir, err := ioutil.TempDir("", "docker-export-")
	if err != nil {
		return job.Error(err)
	}
	defer os.RemoveAll(tempdir)

	utils.Debugf("Serializing %s", name)

	rootRepo, err := srv.daemon.Repositories().Get(name)
	if err != nil {
		return job.Error(err)
	}
	if rootRepo != nil {
		for _, id := range rootRepo {
			if err := srv.exportImage(job.Eng, id, tempdir); err != nil {
				return job.Error(err)
			}
		}

		// write repositories
		rootRepoMap := map[string]graph.Repository{}
		rootRepoMap[name] = rootRepo
		rootRepoJson, _ := json.Marshal(rootRepoMap)

		if err := ioutil.WriteFile(path.Join(tempdir, "repositories"), rootRepoJson, os.FileMode(0644)); err != nil {
			return job.Error(err)
		}
	} else {
		if err := srv.exportImage(job.Eng, name, tempdir); err != nil {
			return job.Error(err)
		}
	}

	fs, err := archive.Tar(tempdir, archive.Uncompressed)
	if err != nil {
		return job.Error(err)
	}
	defer fs.Close()

	if _, err := io.Copy(job.Stdout, fs); err != nil {
		return job.Error(err)
	}
	utils.Debugf("End Serializing %s", name)
	return engine.StatusOK
}

func (srv *Server) exportImage(eng *engine.Engine, name, tempdir string) error {
	for n := name; n != ""; {
		// temporary directory
		tmpImageDir := path.Join(tempdir, n)
		if err := os.Mkdir(tmpImageDir, os.FileMode(0755)); err != nil {
			if os.IsExist(err) {
				return nil
			}
			return err
		}

		var version = "1.0"
		var versionBuf = []byte(version)

		if err := ioutil.WriteFile(path.Join(tmpImageDir, "VERSION"), versionBuf, os.FileMode(0644)); err != nil {
			return err
		}

		// serialize json
		json, err := os.Create(path.Join(tmpImageDir, "json"))
		if err != nil {
			return err
		}
		job := eng.Job("image_inspect", n)
		job.Stdout.Add(json)
		if err := job.Run(); err != nil {
			return err
		}

		// serialize filesystem
		fsTar, err := os.Create(path.Join(tmpImageDir, "layer.tar"))
		if err != nil {
			return err
		}
		job = eng.Job("image_tarlayer", n)
		job.Stdout.Add(fsTar)
		if err := job.Run(); err != nil {
			return err
		}

		// find parent
		job = eng.Job("image_get", n)
		info, _ := job.Stdout.AddEnv()
		if err := job.Run(); err != nil {
			return err
		}
		n = info.Get("Parent")
	}
	return nil
}

func (srv *Server) Build(job *engine.Job) engine.Status {
	if len(job.Args) != 0 {
		return job.Errorf("Usage: %s\n", job.Name)
	}
	var (
		remoteURL      = job.Getenv("remote")
		repoName       = job.Getenv("t")
		suppressOutput = job.GetenvBool("q")
		noCache        = job.GetenvBool("nocache")
		rm             = job.GetenvBool("rm")
		forceRm        = job.GetenvBool("forcerm")
		authConfig     = &registry.AuthConfig{}
		configFile     = &registry.ConfigFile{}
		tag            string
		context        io.ReadCloser
	)
	job.GetenvJson("authConfig", authConfig)
	job.GetenvJson("configFile", configFile)
	repoName, tag = utils.ParseRepositoryTag(repoName)

	if remoteURL == "" {
		context = ioutil.NopCloser(job.Stdin)
	} else if utils.IsGIT(remoteURL) {
		if !strings.HasPrefix(remoteURL, "git://") {
			remoteURL = "https://" + remoteURL
		}
		root, err := ioutil.TempDir("", "docker-build-git")
		if err != nil {
			return job.Error(err)
		}
		defer os.RemoveAll(root)

		if output, err := exec.Command("git", "clone", "--recursive", remoteURL, root).CombinedOutput(); err != nil {
			return job.Errorf("Error trying to use git: %s (%s)", err, output)
		}

		c, err := archive.Tar(root, archive.Uncompressed)
		if err != nil {
			return job.Error(err)
		}
		context = c
	} else if utils.IsURL(remoteURL) {
		f, err := utils.Download(remoteURL)
		if err != nil {
			return job.Error(err)
		}
		defer f.Body.Close()
		dockerFile, err := ioutil.ReadAll(f.Body)
		if err != nil {
			return job.Error(err)
		}
		c, err := archive.Generate("Dockerfile", string(dockerFile))
		if err != nil {
			return job.Error(err)
		}
		context = c
	}
	defer context.Close()

	sf := utils.NewStreamFormatter(job.GetenvBool("json"))
	b := NewBuildFile(srv,
		&utils.StdoutFormater{
			Writer:          job.Stdout,
			StreamFormatter: sf,
		},
		&utils.StderrFormater{
			Writer:          job.Stdout,
			StreamFormatter: sf,
		},
		!suppressOutput, !noCache, rm, forceRm, job.Stdout, sf, authConfig, configFile)
	id, err := b.Build(context)
	if err != nil {
		return job.Error(err)
	}
	if repoName != "" {
		srv.daemon.Repositories().Set(repoName, tag, id, false)
	}
	return engine.StatusOK
}

// Loads a set of images into the repository. This is the complementary of ImageExport.
// The input stream is an uncompressed tar ball containing images and metadata.
func (srv *Server) ImageLoad(job *engine.Job) engine.Status {
	tmpImageDir, err := ioutil.TempDir("", "docker-import-")
	if err != nil {
		return job.Error(err)
	}
	defer os.RemoveAll(tmpImageDir)

	var (
		repoTarFile = path.Join(tmpImageDir, "repo.tar")
		repoDir     = path.Join(tmpImageDir, "repo")
	)

	tarFile, err := os.Create(repoTarFile)
	if err != nil {
		return job.Error(err)
	}
	if _, err := io.Copy(tarFile, job.Stdin); err != nil {
		return job.Error(err)
	}
	tarFile.Close()

	repoFile, err := os.Open(repoTarFile)
	if err != nil {
		return job.Error(err)
	}
	if err := os.Mkdir(repoDir, os.ModeDir); err != nil {
		return job.Error(err)
	}
	if err := archive.Untar(repoFile, repoDir, nil); err != nil {
		return job.Error(err)
	}

	dirs, err := ioutil.ReadDir(repoDir)
	if err != nil {
		return job.Error(err)
	}

	for _, d := range dirs {
		if d.IsDir() {
			if err := srv.recursiveLoad(job.Eng, d.Name(), tmpImageDir); err != nil {
				return job.Error(err)
			}
		}
	}

	repositoriesJson, err := ioutil.ReadFile(path.Join(tmpImageDir, "repo", "repositories"))
	if err == nil {
		repositories := map[string]graph.Repository{}
		if err := json.Unmarshal(repositoriesJson, &repositories); err != nil {
			return job.Error(err)
		}

		for imageName, tagMap := range repositories {
			for tag, address := range tagMap {
				if err := srv.daemon.Repositories().Set(imageName, tag, address, true); err != nil {
					return job.Error(err)
				}
			}
		}
	} else if !os.IsNotExist(err) {
		return job.Error(err)
	}

	return engine.StatusOK
}

func (srv *Server) recursiveLoad(eng *engine.Engine, address, tmpImageDir string) error {
	if err := eng.Job("image_get", address).Run(); err != nil {
		utils.Debugf("Loading %s", address)

		imageJson, err := ioutil.ReadFile(path.Join(tmpImageDir, "repo", address, "json"))
		if err != nil {
			utils.Debugf("Error reading json", err)
			return err
		}

		layer, err := os.Open(path.Join(tmpImageDir, "repo", address, "layer.tar"))
		if err != nil {
			utils.Debugf("Error reading embedded tar", err)
			return err
		}
		img, err := image.NewImgJSON(imageJson)
		if err != nil {
			utils.Debugf("Error unmarshalling json", err)
			return err
		}
		if img.Parent != "" {
			if !srv.daemon.Graph().Exists(img.Parent) {
				if err := srv.recursiveLoad(eng, img.Parent, tmpImageDir); err != nil {
					return err
				}
			}
		}
		if err := srv.daemon.Graph().Register(imageJson, layer, img); err != nil {
			return err
		}
	}
	utils.Debugf("Completed processing %s", address)

	return nil
}

func (srv *Server) ImagesViz(job *engine.Job) engine.Status {
	images, _ := srv.daemon.Graph().Map()
	if images == nil {
		return engine.StatusOK
	}
	job.Stdout.Write([]byte("digraph docker {\n"))

	var (
		parentImage *image.Image
		err         error
	)
	for _, image := range images {
		parentImage, err = image.GetParent()
		if err != nil {
			return job.Errorf("Error while getting parent image: %v", err)
		}
		if parentImage != nil {
			job.Stdout.Write([]byte(" \"" + parentImage.ID + "\" -> \"" + image.ID + "\"\n"))
		} else {
			job.Stdout.Write([]byte(" base -> \"" + image.ID + "\" [style=invis]\n"))
		}
	}

	for id, repos := range srv.daemon.Repositories().GetRepoRefs() {
		job.Stdout.Write([]byte(" \"" + id + "\" [label=\"" + id + "\\n" + strings.Join(repos, "\\n") + "\",shape=box,fillcolor=\"paleturquoise\",style=\"filled,rounded\"];\n"))
	}
	job.Stdout.Write([]byte(" base [style=invisible]\n}\n"))
	return engine.StatusOK
}

func (srv *Server) Images(job *engine.Job) engine.Status {
	var (
		allImages   map[string]*image.Image
		err         error
		filt_tagged = true
	)

	imageFilters, err := filters.FromParam(job.Getenv("filters"))
	if err != nil {
		return job.Error(err)
	}
	if i, ok := imageFilters["dangling"]; ok {
		for _, value := range i {
			if strings.ToLower(value) == "true" {
				filt_tagged = false
			}
		}
	}

	if job.GetenvBool("all") && filt_tagged {
		allImages, err = srv.daemon.Graph().Map()
	} else {
		allImages, err = srv.daemon.Graph().Heads()
	}
	if err != nil {
		return job.Error(err)
	}
	lookup := make(map[string]*engine.Env)
	srv.daemon.Repositories().Lock()
	for name, repository := range srv.daemon.Repositories().Repositories {
		if job.Getenv("filter") != "" {
			if match, _ := path.Match(job.Getenv("filter"), name); !match {
				continue
			}
		}
		for tag, id := range repository {
			image, err := srv.daemon.Graph().Get(id)
			if err != nil {
				log.Printf("Warning: couldn't load %s from %s/%s: %s", id, name, tag, err)
				continue
			}

			if out, exists := lookup[id]; exists {
				if filt_tagged {
					out.SetList("RepoTags", append(out.GetList("RepoTags"), fmt.Sprintf("%s:%s", name, tag)))
				}
			} else {
				// get the boolean list for if only the untagged images are requested
				delete(allImages, id)
				if filt_tagged {
					out := &engine.Env{}
					out.Set("ParentId", image.Parent)
					out.SetList("RepoTags", []string{fmt.Sprintf("%s:%s", name, tag)})
					out.Set("Id", image.ID)
					out.SetInt64("Created", image.Created.Unix())
					out.SetInt64("Size", image.Size)
					out.SetInt64("VirtualSize", image.GetParentsSize(0)+image.Size)
					lookup[id] = out
				}
			}

		}
	}
	srv.daemon.Repositories().Unlock()

	outs := engine.NewTable("Created", len(lookup))
	for _, value := range lookup {
		outs.Add(value)
	}

	// Display images which aren't part of a repository/tag
	if job.Getenv("filter") == "" {
		for _, image := range allImages {
			out := &engine.Env{}
			out.Set("ParentId", image.Parent)
			out.SetList("RepoTags", []string{"<none>:<none>"})
			out.Set("Id", image.ID)
			out.SetInt64("Created", image.Created.Unix())
			out.SetInt64("Size", image.Size)
			out.SetInt64("VirtualSize", image.GetParentsSize(0)+image.Size)
			outs.Add(out)
		}
	}

	outs.ReverseSort()
	if _, err := outs.WriteListTo(job.Stdout); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

func (srv *Server) DockerInfo(job *engine.Job) engine.Status {
	images, _ := srv.daemon.Graph().Map()
	var imgcount int
	if images == nil {
		imgcount = 0
	} else {
		imgcount = len(images)
	}
	kernelVersion := "<unknown>"
	if kv, err := utils.GetKernelVersion(); err == nil {
		kernelVersion = kv.String()
	}

	// if we still have the original dockerinit binary from before we copied it locally, let's return the path to that, since that's more intuitive (the copied path is trivial to derive by hand given VERSION)
	initPath := utils.DockerInitPath("")
	if initPath == "" {
		// if that fails, we'll just return the path from the daemon
		initPath = srv.daemon.SystemInitPath()
	}

	v := &engine.Env{}
	v.SetInt("Containers", len(srv.daemon.List()))
	v.SetInt("Images", imgcount)
	v.Set("Driver", srv.daemon.GraphDriver().String())
	v.SetJson("DriverStatus", srv.daemon.GraphDriver().Status())
	v.SetBool("MemoryLimit", srv.daemon.SystemConfig().MemoryLimit)
	v.SetBool("SwapLimit", srv.daemon.SystemConfig().SwapLimit)
	v.SetBool("IPv4Forwarding", !srv.daemon.SystemConfig().IPv4ForwardingDisabled)
	v.SetBool("Debug", os.Getenv("DEBUG") != "")
	v.SetInt("NFd", utils.GetTotalUsedFds())
	v.SetInt("NGoroutines", runtime.NumGoroutine())
	v.Set("ExecutionDriver", srv.daemon.ExecutionDriver().Name())
	v.SetInt("NEventsListener", len(srv.listeners))
	v.Set("KernelVersion", kernelVersion)
	v.Set("IndexServerAddress", registry.IndexServerAddress())
	v.Set("InitSha1", dockerversion.INITSHA1)
	v.Set("InitPath", initPath)
	if _, err := v.WriteTo(job.Stdout); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

func (srv *Server) ImageHistory(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 1 {
		return job.Errorf("Usage: %s IMAGE", job.Name)
	}
	name := job.Args[0]
	foundImage, err := srv.daemon.Repositories().LookupImage(name)
	if err != nil {
		return job.Error(err)
	}

	lookupMap := make(map[string][]string)
	for name, repository := range srv.daemon.Repositories().Repositories {
		for tag, id := range repository {
			// If the ID already has a reverse lookup, do not update it unless for "latest"
			if _, exists := lookupMap[id]; !exists {
				lookupMap[id] = []string{}
			}
			lookupMap[id] = append(lookupMap[id], name+":"+tag)
		}
	}

	outs := engine.NewTable("Created", 0)
	err = foundImage.WalkHistory(func(img *image.Image) error {
		out := &engine.Env{}
		out.Set("Id", img.ID)
		out.SetInt64("Created", img.Created.Unix())
		out.Set("CreatedBy", strings.Join(img.ContainerConfig.Cmd, " "))
		out.SetList("Tags", lookupMap[img.ID])
		out.SetInt64("Size", img.Size)
		outs.Add(out)
		return nil
	})
	outs.ReverseSort()
	if _, err := outs.WriteListTo(job.Stdout); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

func (srv *Server) ContainerTop(job *engine.Job) engine.Status {
	if len(job.Args) != 1 && len(job.Args) != 2 {
		return job.Errorf("Not enough arguments. Usage: %s CONTAINER [PS_ARGS]\n", job.Name)
	}
	var (
		name   = job.Args[0]
		psArgs = "-ef"
	)

	if len(job.Args) == 2 && job.Args[1] != "" {
		psArgs = job.Args[1]
	}

	if container := srv.daemon.Get(name); container != nil {
		if !container.State.IsRunning() {
			return job.Errorf("Container %s is not running", name)
		}
		pids, err := srv.daemon.ExecutionDriver().GetPidsForContainer(container.ID)
		if err != nil {
			return job.Error(err)
		}
		output, err := exec.Command("ps", psArgs).Output()
		if err != nil {
			return job.Errorf("Error running ps: %s", err)
		}

		lines := strings.Split(string(output), "\n")
		header := strings.Fields(lines[0])
		out := &engine.Env{}
		out.SetList("Titles", header)

		pidIndex := -1
		for i, name := range header {
			if name == "PID" {
				pidIndex = i
			}
		}
		if pidIndex == -1 {
			return job.Errorf("Couldn't find PID field in ps output")
		}

		processes := [][]string{}
		for _, line := range lines[1:] {
			if len(line) == 0 {
				continue
			}
			fields := strings.Fields(line)
			p, err := strconv.Atoi(fields[pidIndex])
			if err != nil {
				return job.Errorf("Unexpected pid '%s': %s", fields[pidIndex], err)
			}

			for _, pid := range pids {
				if pid == p {
					// Make sure number of fields equals number of header titles
					// merging "overhanging" fields
					process := fields[:len(header)-1]
					process = append(process, strings.Join(fields[len(header)-1:], " "))
					processes = append(processes, process)
				}
			}
		}
		out.SetJson("Processes", processes)
		out.WriteTo(job.Stdout)
		return engine.StatusOK

	}
	return job.Errorf("No such container: %s", name)
}

func (srv *Server) ContainerChanges(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 1 {
		return job.Errorf("Usage: %s CONTAINER", job.Name)
	}
	name := job.Args[0]
	if container := srv.daemon.Get(name); container != nil {
		outs := engine.NewTable("", 0)
		changes, err := container.Changes()
		if err != nil {
			return job.Error(err)
		}
		for _, change := range changes {
			out := &engine.Env{}
			if err := out.Import(change); err != nil {
				return job.Error(err)
			}
			outs.Add(out)
		}
		if _, err := outs.WriteListTo(job.Stdout); err != nil {
			return job.Error(err)
		}
	} else {
		return job.Errorf("No such container: %s", name)
	}
	return engine.StatusOK
}

func (srv *Server) Containers(job *engine.Job) engine.Status {
	var (
		foundBefore bool
		displayed   int
		all         = job.GetenvBool("all")
		since       = job.Getenv("since")
		before      = job.Getenv("before")
		n           = job.GetenvInt("limit")
		size        = job.GetenvBool("size")
	)
	outs := engine.NewTable("Created", 0)

	names := map[string][]string{}
	srv.daemon.ContainerGraph().Walk("/", func(p string, e *graphdb.Entity) error {
		names[e.ID()] = append(names[e.ID()], p)
		return nil
	}, -1)

	var beforeCont, sinceCont *daemon.Container
	if before != "" {
		beforeCont = srv.daemon.Get(before)
		if beforeCont == nil {
			return job.Error(fmt.Errorf("Could not find container with name or id %s", before))
		}
	}

	if since != "" {
		sinceCont = srv.daemon.Get(since)
		if sinceCont == nil {
			return job.Error(fmt.Errorf("Could not find container with name or id %s", since))
		}
	}

	for _, container := range srv.daemon.List() {
		if !container.State.IsRunning() && !all && n <= 0 && since == "" && before == "" {
			continue
		}
		if before != "" && !foundBefore {
			if container.ID == beforeCont.ID {
				foundBefore = true
			}
			continue
		}
		if n > 0 && displayed == n {
			break
		}
		if since != "" {
			if container.ID == sinceCont.ID {
				break
			}
		}
		displayed++
		out := &engine.Env{}
		out.Set("Id", container.ID)
		out.SetList("Names", names[container.ID])
		out.Set("Image", srv.daemon.Repositories().ImageName(container.Image))
		if len(container.Args) > 0 {
			args := []string{}
			for _, arg := range container.Args {
				if strings.Contains(arg, " ") {
					args = append(args, fmt.Sprintf("'%s'", arg))
				} else {
					args = append(args, arg)
				}
			}
			argsAsString := strings.Join(args, " ")

			out.Set("Command", fmt.Sprintf("\"%s %s\"", container.Path, argsAsString))
		} else {
			out.Set("Command", fmt.Sprintf("\"%s\"", container.Path))
		}
		out.SetInt64("Created", container.Created.Unix())
		out.Set("Status", container.State.String())
		str, err := container.NetworkSettings.PortMappingAPI().ToListString()
		if err != nil {
			return job.Error(err)
		}
		out.Set("Ports", str)
		if size {
			sizeRw, sizeRootFs := container.GetSize()
			out.SetInt64("SizeRw", sizeRw)
			out.SetInt64("SizeRootFs", sizeRootFs)
		}
		outs.Add(out)
	}
	outs.ReverseSort()
	if _, err := outs.WriteListTo(job.Stdout); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

func (srv *Server) ContainerCommit(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Not enough arguments. Usage: %s CONTAINER\n", job.Name)
	}
	name := job.Args[0]

	container := srv.daemon.Get(name)
	if container == nil {
		return job.Errorf("No such container: %s", name)
	}

	var (
		config    = container.Config
		newConfig runconfig.Config
	)

	if err := job.GetenvJson("config", &newConfig); err != nil {
		return job.Error(err)
	}

	if err := runconfig.Merge(&newConfig, config); err != nil {
		return job.Error(err)
	}

	img, err := srv.daemon.Commit(container, job.Getenv("repo"), job.Getenv("tag"), job.Getenv("comment"), job.Getenv("author"), &newConfig)
	if err != nil {
		return job.Error(err)
	}
	job.Printf("%s\n", img.ID)
	return engine.StatusOK
}

func (srv *Server) ImageTag(job *engine.Job) engine.Status {
	if len(job.Args) != 2 && len(job.Args) != 3 {
		return job.Errorf("Usage: %s IMAGE REPOSITORY [TAG]\n", job.Name)
	}
	var tag string
	if len(job.Args) == 3 {
		tag = job.Args[2]
	}
	if err := srv.daemon.Repositories().Set(job.Args[1], tag, job.Args[0], job.GetenvBool("force")); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

func (srv *Server) pullImage(r *registry.Registry, out io.Writer, imgID, endpoint string, token []string, sf *utils.StreamFormatter) error {
	history, err := r.GetRemoteHistory(imgID, endpoint, token)
	if err != nil {
		return err
	}
	out.Write(sf.FormatProgress(utils.TruncateID(imgID), "Pulling dependent layers", nil))
	// FIXME: Try to stream the images?
	// FIXME: Launch the getRemoteImage() in goroutines

	for i := len(history) - 1; i >= 0; i-- {
		id := history[i]

		// ensure no two downloads of the same layer happen at the same time
		if c, err := srv.poolAdd("pull", "layer:"+id); err != nil {
			utils.Debugf("Image (id: %s) pull is already running, skipping: %v", id, err)
			<-c
		}
		defer srv.poolRemove("pull", "layer:"+id)

		if !srv.daemon.Graph().Exists(id) {
			out.Write(sf.FormatProgress(utils.TruncateID(id), "Pulling metadata", nil))
			var (
				imgJSON []byte
				imgSize int
				err     error
				img     *image.Image
			)
			retries := 5
			for j := 1; j <= retries; j++ {
				imgJSON, imgSize, err = r.GetRemoteImageJSON(id, endpoint, token)
				if err != nil && j == retries {
					out.Write(sf.FormatProgress(utils.TruncateID(id), "Error pulling dependent layers", nil))
					return err
				} else if err != nil {
					time.Sleep(time.Duration(j) * 500 * time.Millisecond)
					continue
				}
				img, err = image.NewImgJSON(imgJSON)
				if err != nil && j == retries {
					out.Write(sf.FormatProgress(utils.TruncateID(id), "Error pulling dependent layers", nil))
					return fmt.Errorf("Failed to parse json: %s", err)
				} else if err != nil {
					time.Sleep(time.Duration(j) * 500 * time.Millisecond)
					continue
				} else {
					break
				}
			}

			for j := 1; j <= retries; j++ {
				// Get the layer
				status := "Pulling fs layer"
				if j > 1 {
					status = fmt.Sprintf("Pulling fs layer [retries: %d]", j)
				}
				out.Write(sf.FormatProgress(utils.TruncateID(id), status, nil))
				layer, err := r.GetRemoteImageLayer(img.ID, endpoint, token, int64(imgSize))
				if uerr, ok := err.(*url.Error); ok {
					err = uerr.Err
				}
				if terr, ok := err.(net.Error); ok && terr.Timeout() && j < retries {
					time.Sleep(time.Duration(j) * 500 * time.Millisecond)
					continue
				} else if err != nil {
					out.Write(sf.FormatProgress(utils.TruncateID(id), "Error pulling dependent layers", nil))
					return err
				}
				defer layer.Close()

				err = srv.daemon.Graph().Register(imgJSON,
					utils.ProgressReader(layer, imgSize, out, sf, false, utils.TruncateID(id), "Downloading"),
					img)
				if terr, ok := err.(net.Error); ok && terr.Timeout() && j < retries {
					time.Sleep(time.Duration(j) * 500 * time.Millisecond)
					continue
				} else if err != nil {
					out.Write(sf.FormatProgress(utils.TruncateID(id), "Error downloading dependent layers", nil))
					return err
				} else {
					break
				}
			}
		}
		out.Write(sf.FormatProgress(utils.TruncateID(id), "Download complete", nil))

	}
	return nil
}

func (srv *Server) pullRepository(r *registry.Registry, out io.Writer, localName, remoteName, askedTag string, sf *utils.StreamFormatter, parallel bool) error {
	out.Write(sf.FormatStatus("", "Pulling repository %s", localName))

	repoData, err := r.GetRepositoryData(remoteName)
	if err != nil {
		return err
	}

	utils.Debugf("Retrieving the tag list")
	tagsList, err := r.GetRemoteTags(repoData.Endpoints, remoteName, repoData.Tokens)
	if err != nil {
		utils.Errorf("%v", err)
		return err
	}

	for tag, id := range tagsList {
		repoData.ImgList[id] = &registry.ImgData{
			ID:       id,
			Tag:      tag,
			Checksum: "",
		}
	}

	utils.Debugf("Registering tags")
	// If no tag has been specified, pull them all
	if askedTag == "" {
		for tag, id := range tagsList {
			repoData.ImgList[id].Tag = tag
		}
	} else {
		// Otherwise, check that the tag exists and use only that one
		id, exists := tagsList[askedTag]
		if !exists {
			return fmt.Errorf("Tag %s not found in repository %s", askedTag, localName)
		}
		repoData.ImgList[id].Tag = askedTag
	}

	errors := make(chan error)
	for _, image := range repoData.ImgList {
		downloadImage := func(img *registry.ImgData) {
			if askedTag != "" && img.Tag != askedTag {
				utils.Debugf("(%s) does not match %s (id: %s), skipping", img.Tag, askedTag, img.ID)
				if parallel {
					errors <- nil
				}
				return
			}

			if img.Tag == "" {
				utils.Debugf("Image (id: %s) present in this repository but untagged, skipping", img.ID)
				if parallel {
					errors <- nil
				}
				return
			}

			// ensure no two downloads of the same image happen at the same time
			if c, err := srv.poolAdd("pull", "img:"+img.ID); err != nil {
				if c != nil {
					out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Layer already being pulled by another client. Waiting.", nil))
					<-c
					out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Download complete", nil))
				} else {
					utils.Debugf("Image (id: %s) pull is already running, skipping: %v", img.ID, err)
				}
				if parallel {
					errors <- nil
				}
				return
			}
			defer srv.poolRemove("pull", "img:"+img.ID)

			out.Write(sf.FormatProgress(utils.TruncateID(img.ID), fmt.Sprintf("Pulling image (%s) from %s", img.Tag, localName), nil))
			success := false
			var lastErr error
			for _, ep := range repoData.Endpoints {
				out.Write(sf.FormatProgress(utils.TruncateID(img.ID), fmt.Sprintf("Pulling image (%s) from %s, endpoint: %s", img.Tag, localName, ep), nil))
				if err := srv.pullImage(r, out, img.ID, ep, repoData.Tokens, sf); err != nil {
					// It's not ideal that only the last error is returned, it would be better to concatenate the errors.
					// As the error is also given to the output stream the user will see the error.
					lastErr = err
					out.Write(sf.FormatProgress(utils.TruncateID(img.ID), fmt.Sprintf("Error pulling image (%s) from %s, endpoint: %s, %s", img.Tag, localName, ep, err), nil))
					continue
				}
				success = true
				break
			}
			if !success {
				out.Write(sf.FormatProgress(utils.TruncateID(img.ID), fmt.Sprintf("Error pulling image (%s) from %s, %s", img.Tag, localName, lastErr), nil))
				if parallel {
					errors <- fmt.Errorf("Could not find repository on any of the indexed registries.")
					return
				}
			}
			out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Download complete", nil))

			if parallel {
				errors <- nil
			}
		}

		if parallel {
			go downloadImage(image)
		} else {
			downloadImage(image)
		}
	}
	if parallel {
		var lastError error
		for i := 0; i < len(repoData.ImgList); i++ {
			if err := <-errors; err != nil {
				lastError = err
			}
		}
		if lastError != nil {
			return lastError
		}

	}
	for tag, id := range tagsList {
		if askedTag != "" && tag != askedTag {
			continue
		}
		if err := srv.daemon.Repositories().Set(localName, tag, id, true); err != nil {
			return err
		}
	}

	return nil
}

func (srv *Server) poolAdd(kind, key string) (chan struct{}, error) {
	srv.Lock()
	defer srv.Unlock()

	if c, exists := srv.pullingPool[key]; exists {
		return c, fmt.Errorf("pull %s is already in progress", key)
	}
	if c, exists := srv.pushingPool[key]; exists {
		return c, fmt.Errorf("push %s is already in progress", key)
	}

	c := make(chan struct{})
	switch kind {
	case "pull":
		srv.pullingPool[key] = c
	case "push":
		srv.pushingPool[key] = c
	default:
		return nil, fmt.Errorf("Unknown pool type")
	}
	return c, nil
}

func (srv *Server) poolRemove(kind, key string) error {
	srv.Lock()
	defer srv.Unlock()
	switch kind {
	case "pull":
		if c, exists := srv.pullingPool[key]; exists {
			close(c)
			delete(srv.pullingPool, key)
		}
	case "push":
		if c, exists := srv.pushingPool[key]; exists {
			close(c)
			delete(srv.pushingPool, key)
		}
	default:
		return fmt.Errorf("Unknown pool type")
	}
	return nil
}

func (srv *Server) ImagePull(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 1 && n != 2 {
		return job.Errorf("Usage: %s IMAGE [TAG]", job.Name)
	}
	var (
		localName   = job.Args[0]
		tag         string
		sf          = utils.NewStreamFormatter(job.GetenvBool("json"))
		authConfig  = &registry.AuthConfig{}
		metaHeaders map[string][]string
	)
	if len(job.Args) > 1 {
		tag = job.Args[1]
	}

	job.GetenvJson("authConfig", authConfig)
	job.GetenvJson("metaHeaders", &metaHeaders)

	c, err := srv.poolAdd("pull", localName+":"+tag)
	if err != nil {
		if c != nil {
			// Another pull of the same repository is already taking place; just wait for it to finish
			job.Stdout.Write(sf.FormatStatus("", "Repository %s already being pulled by another client. Waiting.", localName))
			<-c
			return engine.StatusOK
		}
		return job.Error(err)
	}
	defer srv.poolRemove("pull", localName+":"+tag)

	// Resolve the Repository name from fqn to endpoint + name
	hostname, remoteName, err := registry.ResolveRepositoryName(localName)
	if err != nil {
		return job.Error(err)
	}

	endpoint, err := registry.ExpandAndVerifyRegistryUrl(hostname)
	if err != nil {
		return job.Error(err)
	}

	r, err := registry.NewRegistry(authConfig, registry.HTTPRequestFactory(metaHeaders), endpoint)
	if err != nil {
		return job.Error(err)
	}

	if endpoint == registry.IndexServerAddress() {
		// If pull "index.docker.io/foo/bar", it's stored locally under "foo/bar"
		localName = remoteName
	}

	if err = srv.pullRepository(r, job.Stdout, localName, remoteName, tag, sf, job.GetenvBool("parallel")); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}

// Retrieve the all the images to be uploaded in the correct order
func (srv *Server) getImageList(localRepo map[string]string, requestedTag string) ([]string, map[string][]string, error) {
	var (
		imageList   []string
		imagesSeen  map[string]bool     = make(map[string]bool)
		tagsByImage map[string][]string = make(map[string][]string)
	)

	for tag, id := range localRepo {
		if requestedTag != "" && requestedTag != tag {
			continue
		}
		var imageListForThisTag []string

		tagsByImage[id] = append(tagsByImage[id], tag)

		for img, err := srv.daemon.Graph().Get(id); img != nil; img, err = img.GetParent() {
			if err != nil {
				return nil, nil, err
			}

			if imagesSeen[img.ID] {
				// This image is already on the list, we can ignore it and all its parents
				break
			}

			imagesSeen[img.ID] = true
			imageListForThisTag = append(imageListForThisTag, img.ID)
		}

		// reverse the image list for this tag (so the "most"-parent image is first)
		for i, j := 0, len(imageListForThisTag)-1; i < j; i, j = i+1, j-1 {
			imageListForThisTag[i], imageListForThisTag[j] = imageListForThisTag[j], imageListForThisTag[i]
		}

		// append to main image list
		imageList = append(imageList, imageListForThisTag...)
	}
	if len(imageList) == 0 {
		return nil, nil, fmt.Errorf("No images found for the requested repository / tag")
	}
	utils.Debugf("Image list: %v", imageList)
	utils.Debugf("Tags by image: %v", tagsByImage)

	return imageList, tagsByImage, nil
}

func (srv *Server) pushRepository(r *registry.Registry, out io.Writer, localName, remoteName string, localRepo map[string]string, tag string, sf *utils.StreamFormatter) error {
	out = utils.NewWriteFlusher(out)
	utils.Debugf("Local repo: %s", localRepo)
	imgList, tagsByImage, err := srv.getImageList(localRepo, tag)
	if err != nil {
		return err
	}

	out.Write(sf.FormatStatus("", "Sending image list"))

	var (
		repoData   *registry.RepositoryData
		imageIndex []*registry.ImgData
	)

	for _, imgId := range imgList {
		if tags, exists := tagsByImage[imgId]; exists {
			// If an image has tags you must add an entry in the image index
			// for each tag
			for _, tag := range tags {
				imageIndex = append(imageIndex, &registry.ImgData{
					ID:  imgId,
					Tag: tag,
				})
			}
		} else {
			// If the image does not have a tag it still needs to be sent to the
			// registry with an empty tag so that it is accociated with the repository
			imageIndex = append(imageIndex, &registry.ImgData{
				ID:  imgId,
				Tag: "",
			})

		}
	}

	utils.Debugf("Preparing to push %s with the following images and tags\n", localRepo)
	for _, data := range imageIndex {
		utils.Debugf("Pushing ID: %s with Tag: %s\n", data.ID, data.Tag)
	}

	// Register all the images in a repository with the registry
	// If an image is not in this list it will not be associated with the repository
	repoData, err = r.PushImageJSONIndex(remoteName, imageIndex, false, nil)
	if err != nil {
		return err
	}

	nTag := 1
	if tag == "" {
		nTag = len(localRepo)
	}
	for _, ep := range repoData.Endpoints {
		out.Write(sf.FormatStatus("", "Pushing repository %s (%d tags)", localName, nTag))

		for _, imgId := range imgList {
			if r.LookupRemoteImage(imgId, ep, repoData.Tokens) {
				out.Write(sf.FormatStatus("", "Image %s already pushed, skipping", utils.TruncateID(imgId)))
			} else {
				if _, err := srv.pushImage(r, out, remoteName, imgId, ep, repoData.Tokens, sf); err != nil {
					// FIXME: Continue on error?
					return err
				}
			}

			for _, tag := range tagsByImage[imgId] {
				out.Write(sf.FormatStatus("", "Pushing tag for rev [%s] on {%s}", utils.TruncateID(imgId), ep+"repositories/"+remoteName+"/tags/"+tag))

				if err := r.PushRegistryTag(remoteName, imgId, tag, ep, repoData.Tokens); err != nil {
					return err
				}
			}
		}
	}

	if _, err := r.PushImageJSONIndex(remoteName, imageIndex, true, repoData.Endpoints); err != nil {
		return err
	}

	return nil
}

func (srv *Server) pushImage(r *registry.Registry, out io.Writer, remote, imgID, ep string, token []string, sf *utils.StreamFormatter) (checksum string, err error) {
	out = utils.NewWriteFlusher(out)
	jsonRaw, err := ioutil.ReadFile(path.Join(srv.daemon.Graph().Root, imgID, "json"))
	if err != nil {
		return "", fmt.Errorf("Cannot retrieve the path for {%s}: %s", imgID, err)
	}
	out.Write(sf.FormatProgress(utils.TruncateID(imgID), "Pushing", nil))

	imgData := &registry.ImgData{
		ID: imgID,
	}

	// Send the json
	if err := r.PushImageJSONRegistry(imgData, jsonRaw, ep, token); err != nil {
		if err == registry.ErrAlreadyExists {
			out.Write(sf.FormatProgress(utils.TruncateID(imgData.ID), "Image already pushed, skipping", nil))
			return "", nil
		}
		return "", err
	}

	layerData, err := srv.daemon.Graph().TempLayerArchive(imgID, archive.Uncompressed, sf, out)
	if err != nil {
		return "", fmt.Errorf("Failed to generate layer archive: %s", err)
	}
	defer os.RemoveAll(layerData.Name())

	// Send the layer
	utils.Debugf("rendered layer for %s of [%d] size", imgData.ID, layerData.Size)

	checksum, checksumPayload, err := r.PushImageLayerRegistry(imgData.ID, utils.ProgressReader(layerData, int(layerData.Size), out, sf, false, utils.TruncateID(imgData.ID), "Pushing"), ep, token, jsonRaw)
	if err != nil {
		return "", err
	}
	imgData.Checksum = checksum
	imgData.ChecksumPayload = checksumPayload
	// Send the checksum
	if err := r.PushImageChecksumRegistry(imgData, ep, token); err != nil {
		return "", err
	}

	out.Write(sf.FormatProgress(utils.TruncateID(imgData.ID), "Image successfully pushed", nil))
	return imgData.Checksum, nil
}

// FIXME: Allow to interrupt current push when new push of same image is done.
func (srv *Server) ImagePush(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 1 {
		return job.Errorf("Usage: %s IMAGE", job.Name)
	}
	var (
		localName   = job.Args[0]
		sf          = utils.NewStreamFormatter(job.GetenvBool("json"))
		authConfig  = &registry.AuthConfig{}
		metaHeaders map[string][]string
	)

	tag := job.Getenv("tag")
	job.GetenvJson("authConfig", authConfig)
	job.GetenvJson("metaHeaders", &metaHeaders)
	if _, err := srv.poolAdd("push", localName); err != nil {
		return job.Error(err)
	}
	defer srv.poolRemove("push", localName)

	// Resolve the Repository name from fqn to endpoint + name
	hostname, remoteName, err := registry.ResolveRepositoryName(localName)
	if err != nil {
		return job.Error(err)
	}

	endpoint, err := registry.ExpandAndVerifyRegistryUrl(hostname)
	if err != nil {
		return job.Error(err)
	}

	img, err := srv.daemon.Graph().Get(localName)
	r, err2 := registry.NewRegistry(authConfig, registry.HTTPRequestFactory(metaHeaders), endpoint)
	if err2 != nil {
		return job.Error(err2)
	}

	if err != nil {
		reposLen := 1
		if tag == "" {
			reposLen = len(srv.daemon.Repositories().Repositories[localName])
		}
		job.Stdout.Write(sf.FormatStatus("", "The push refers to a repository [%s] (len: %d)", localName, reposLen))
		// If it fails, try to get the repository
		if localRepo, exists := srv.daemon.Repositories().Repositories[localName]; exists {
			if err := srv.pushRepository(r, job.Stdout, localName, remoteName, localRepo, tag, sf); err != nil {
				return job.Error(err)
			}
			return engine.StatusOK
		}
		return job.Error(err)
	}

	var token []string
	job.Stdout.Write(sf.FormatStatus("", "The push refers to an image: [%s]", localName))
	if _, err := srv.pushImage(r, job.Stdout, remoteName, img.ID, endpoint, token, sf); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

func (srv *Server) ImageImport(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 2 && n != 3 {
		return job.Errorf("Usage: %s SRC REPO [TAG]", job.Name)
	}
	var (
		src     = job.Args[0]
		repo    = job.Args[1]
		tag     string
		sf      = utils.NewStreamFormatter(job.GetenvBool("json"))
		archive archive.ArchiveReader
		resp    *http.Response
	)
	if len(job.Args) > 2 {
		tag = job.Args[2]
	}

	if src == "-" {
		archive = job.Stdin
	} else {
		u, err := url.Parse(src)
		if err != nil {
			return job.Error(err)
		}
		if u.Scheme == "" {
			u.Scheme = "http"
			u.Host = src
			u.Path = ""
		}
		job.Stdout.Write(sf.FormatStatus("", "Downloading from %s", u))
		// Download with curl (pretty progress bar)
		// If curl is not available, fallback to http.Get()
		resp, err = utils.Download(u.String())
		if err != nil {
			return job.Error(err)
		}
		progressReader := utils.ProgressReader(resp.Body, int(resp.ContentLength), job.Stdout, sf, true, "", "Importing")
		defer progressReader.Close()
		archive = progressReader
	}
	img, err := srv.daemon.Graph().Create(archive, "", "", "Imported from "+src, "", nil, nil)
	if err != nil {
		return job.Error(err)
	}
	// Optionally register the image at REPO/TAG
	if repo != "" {
		if err := srv.daemon.Repositories().Set(repo, tag, img.ID, true); err != nil {
			return job.Error(err)
		}
	}
	job.Stdout.Write(sf.FormatStatus("", img.ID))
	return engine.StatusOK
}

func (srv *Server) ContainerCreate(job *engine.Job) engine.Status {
	var name string
	if len(job.Args) == 1 {
		name = job.Args[0]
	} else if len(job.Args) > 1 {
		return job.Errorf("Usage: %s", job.Name)
	}
	config := runconfig.ContainerConfigFromJob(job)
	if config.Memory != 0 && config.Memory < 524288 {
		return job.Errorf("Minimum memory limit allowed is 512k")
	}
	if config.Memory > 0 && !srv.daemon.SystemConfig().MemoryLimit {
		job.Errorf("Your kernel does not support memory limit capabilities. Limitation discarded.\n")
		config.Memory = 0
	}
	if config.Memory > 0 && !srv.daemon.SystemConfig().SwapLimit {
		job.Errorf("Your kernel does not support swap limit capabilities. Limitation discarded.\n")
		config.MemorySwap = -1
	}
	container, buildWarnings, err := srv.daemon.Create(config, name)
	if err != nil {
		if srv.daemon.Graph().IsNotExist(err) {
			_, tag := utils.ParseRepositoryTag(config.Image)
			if tag == "" {
				tag = graph.DEFAULTTAG
			}
			return job.Errorf("No such image: %s (tag: %s)", config.Image, tag)
		}
		return job.Error(err)
	}
	if !container.Config.NetworkDisabled && srv.daemon.SystemConfig().IPv4ForwardingDisabled {
		job.Errorf("IPv4 forwarding is disabled.\n")
	}
	srv.LogEvent("create", container.ID, srv.daemon.Repositories().ImageName(container.Image))
	// FIXME: this is necessary because daemon.Create might return a nil container
	// with a non-nil error. This should not happen! Once it's fixed we
	// can remove this workaround.
	if container != nil {
		job.Printf("%s\n", container.ID)
	}
	for _, warning := range buildWarnings {
		job.Errorf("%s\n", warning)
	}
	return engine.StatusOK
}

func (srv *Server) ContainerRestart(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}
	var (
		name = job.Args[0]
		t    = 10
	)
	if job.EnvExists("t") {
		t = job.GetenvInt("t")
	}
	if container := srv.daemon.Get(name); container != nil {
		if err := container.Restart(int(t)); err != nil {
			return job.Errorf("Cannot restart container %s: %s\n", name, err)
		}
		srv.LogEvent("restart", container.ID, srv.daemon.Repositories().ImageName(container.Image))
	} else {
		return job.Errorf("No such container: %s\n", name)
	}
	return engine.StatusOK
}

func (srv *Server) ContainerDestroy(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Not enough arguments. Usage: %s CONTAINER\n", job.Name)
	}
	name := job.Args[0]
	removeVolume := job.GetenvBool("removeVolume")
	removeLink := job.GetenvBool("removeLink")
	forceRemove := job.GetenvBool("forceRemove")

	container := srv.daemon.Get(name)

	if removeLink {
		if container == nil {
			return job.Errorf("No such link: %s", name)
		}
		name, err := daemon.GetFullContainerName(name)
		if err != nil {
			job.Error(err)
		}
		parent, n := path.Split(name)
		if parent == "/" {
			return job.Errorf("Conflict, cannot remove the default name of the container")
		}
		pe := srv.daemon.ContainerGraph().Get(parent)
		if pe == nil {
			return job.Errorf("Cannot get parent %s for name %s", parent, name)
		}
		parentContainer := srv.daemon.Get(pe.ID())

		if parentContainer != nil {
			parentContainer.DisableLink(n)
		}

		if err := srv.daemon.ContainerGraph().Delete(name); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
	}

	if container != nil {
		if container.State.IsRunning() {
			if forceRemove {
				if err := container.Stop(5); err != nil {
					return job.Errorf("Could not stop running container, cannot remove - %v", err)
				}
			} else {
				return job.Errorf("Impossible to remove a running container, please stop it first or use -f")
			}
		}
		if err := srv.daemon.Destroy(container); err != nil {
			return job.Errorf("Cannot destroy container %s: %s", name, err)
		}
		srv.LogEvent("destroy", container.ID, srv.daemon.Repositories().ImageName(container.Image))

		if removeVolume {
			var (
				volumes     = make(map[string]struct{})
				binds       = make(map[string]struct{})
				usedVolumes = make(map[string]*daemon.Container)
			)

			// the volume id is always the base of the path
			getVolumeId := func(p string) string {
				return filepath.Base(strings.TrimSuffix(p, "/layer"))
			}

			// populate bind map so that they can be skipped and not removed
			for _, bind := range container.HostConfig().Binds {
				source := strings.Split(bind, ":")[0]
				// TODO: refactor all volume stuff, all of it
				// it is very important that we eval the link or comparing the keys to container.Volumes will not work
				//
				// eval symlink can fail, ref #5244 if we receive an is not exist error we can ignore it
				p, err := filepath.EvalSymlinks(source)
				if err != nil && !os.IsNotExist(err) {
					return job.Error(err)
				}
				if p != "" {
					source = p
				}
				binds[source] = struct{}{}
			}

			// Store all the deleted containers volumes
			for _, volumeId := range container.Volumes {
				// Skip the volumes mounted from external
				// bind mounts here will will be evaluated for a symlink
				if _, exists := binds[volumeId]; exists {
					continue
				}

				volumeId = getVolumeId(volumeId)
				volumes[volumeId] = struct{}{}
			}

			// Retrieve all volumes from all remaining containers
			for _, container := range srv.daemon.List() {
				for _, containerVolumeId := range container.Volumes {
					containerVolumeId = getVolumeId(containerVolumeId)
					usedVolumes[containerVolumeId] = container
				}
			}

			for volumeId := range volumes {
				// If the requested volu
				if c, exists := usedVolumes[volumeId]; exists {
					log.Printf("The volume %s is used by the container %s. Impossible to remove it. Skipping.\n", volumeId, c.ID)
					continue
				}
				if err := srv.daemon.Volumes().Delete(volumeId); err != nil {
					return job.Errorf("Error calling volumes.Delete(%q): %v", volumeId, err)
				}
			}
		}
	} else {
		return job.Errorf("No such container: %s", name)
	}
	return engine.StatusOK
}

func (srv *Server) DeleteImage(name string, imgs *engine.Table, first, force, noprune bool) error {
	var (
		repoName, tag string
		tags          = []string{}
		tagDeleted    bool
	)

	repoName, tag = utils.ParseRepositoryTag(name)
	if tag == "" {
		tag = graph.DEFAULTTAG
	}

	img, err := srv.daemon.Repositories().LookupImage(name)
	if err != nil {
		if r, _ := srv.daemon.Repositories().Get(repoName); r != nil {
			return fmt.Errorf("No such image: %s:%s", repoName, tag)
		}
		return fmt.Errorf("No such image: %s", name)
	}

	if strings.Contains(img.ID, name) {
		repoName = ""
		tag = ""
	}

	byParents, err := srv.daemon.Graph().ByParent()
	if err != nil {
		return err
	}

	//If delete by id, see if the id belong only to one repository
	if repoName == "" {
		for _, repoAndTag := range srv.daemon.Repositories().ByID()[img.ID] {
			parsedRepo, parsedTag := utils.ParseRepositoryTag(repoAndTag)
			if repoName == "" || repoName == parsedRepo {
				repoName = parsedRepo
				if parsedTag != "" {
					tags = append(tags, parsedTag)
				}
			} else if repoName != parsedRepo && !force {
				// the id belongs to multiple repos, like base:latest and user:test,
				// in that case return conflict
				return fmt.Errorf("Conflict, cannot delete image %s because it is tagged in multiple repositories, use -f to force", name)
			}
		}
	} else {
		tags = append(tags, tag)
	}

	if !first && len(tags) > 0 {
		return nil
	}

	//Untag the current image
	for _, tag := range tags {
		tagDeleted, err = srv.daemon.Repositories().Delete(repoName, tag)
		if err != nil {
			return err
		}
		if tagDeleted {
			out := &engine.Env{}
			out.Set("Untagged", repoName+":"+tag)
			imgs.Add(out)
			srv.LogEvent("untag", img.ID, "")
		}
	}
	tags = srv.daemon.Repositories().ByID()[img.ID]
	if (len(tags) <= 1 && repoName == "") || len(tags) == 0 {
		if len(byParents[img.ID]) == 0 {
			if err := srv.canDeleteImage(img.ID, force, tagDeleted); err != nil {
				return err
			}
			if err := srv.daemon.Repositories().DeleteAll(img.ID); err != nil {
				return err
			}
			if err := srv.daemon.Graph().Delete(img.ID); err != nil {
				return err
			}
			out := &engine.Env{}
			out.Set("Deleted", img.ID)
			imgs.Add(out)
			srv.LogEvent("delete", img.ID, "")
			if img.Parent != "" && !noprune {
				err := srv.DeleteImage(img.Parent, imgs, false, force, noprune)
				if first {
					return err
				}

			}

		}
	}
	return nil
}

func (srv *Server) ImageDelete(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 1 {
		return job.Errorf("Usage: %s IMAGE", job.Name)
	}
	imgs := engine.NewTable("", 0)
	if err := srv.DeleteImage(job.Args[0], imgs, true, job.GetenvBool("force"), job.GetenvBool("noprune")); err != nil {
		return job.Error(err)
	}
	if len(imgs.Data) == 0 {
		return job.Errorf("Conflict, %s wasn't deleted", job.Args[0])
	}
	if _, err := imgs.WriteListTo(job.Stdout); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

func (srv *Server) canDeleteImage(imgID string, force, untagged bool) error {
	var message string
	if untagged {
		message = " (docker untagged the image)"
	}
	for _, container := range srv.daemon.List() {
		parent, err := srv.daemon.Repositories().LookupImage(container.Image)
		if err != nil {
			return err
		}

		if err := parent.WalkHistory(func(p *image.Image) error {
			if imgID == p.ID {
				if container.State.IsRunning() {
					if force {
						return fmt.Errorf("Conflict, cannot force delete %s because the running container %s is using it%s, stop it and retry", utils.TruncateID(imgID), utils.TruncateID(container.ID), message)
					}
					return fmt.Errorf("Conflict, cannot delete %s because the running container %s is using it%s, stop it and use -f to force", utils.TruncateID(imgID), utils.TruncateID(container.ID), message)
				} else if !force {
					return fmt.Errorf("Conflict, cannot delete %s because the container %s is using it%s, use -f to force", utils.TruncateID(imgID), utils.TruncateID(container.ID), message)
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (srv *Server) ImageGetCached(imgID string, config *runconfig.Config) (*image.Image, error) {
	// Retrieve all images
	images, err := srv.daemon.Graph().Map()
	if err != nil {
		return nil, err
	}

	// Store the tree in a map of map (map[parentId][childId])
	imageMap := make(map[string]map[string]struct{})
	for _, img := range images {
		if _, exists := imageMap[img.Parent]; !exists {
			imageMap[img.Parent] = make(map[string]struct{})
		}
		imageMap[img.Parent][img.ID] = struct{}{}
	}

	// Loop on the children of the given image and check the config
	var match *image.Image
	for elem := range imageMap[imgID] {
		img, err := srv.daemon.Graph().Get(elem)
		if err != nil {
			return nil, err
		}
		if runconfig.Compare(&img.ContainerConfig, config) {
			if match == nil || match.Created.Before(img.Created) {
				match = img
			}
		}
	}
	return match, nil
}

func (srv *Server) ContainerStart(job *engine.Job) engine.Status {
	if len(job.Args) < 1 {
		return job.Errorf("Usage: %s container_id", job.Name)
	}
	var (
		name      = job.Args[0]
		daemon    = srv.daemon
		container = daemon.Get(name)
	)

	if container == nil {
		return job.Errorf("No such container: %s", name)
	}
	// If no environment was set, then no hostconfig was passed.
	if len(job.Environ()) > 0 {
		hostConfig := runconfig.ContainerHostConfigFromJob(job)
		// Validate the HostConfig binds. Make sure that:
		// 1) the source of a bind mount isn't /
		//         The bind mount "/:/foo" isn't allowed.
		// 2) Check that the source exists
		//        The source to be bind mounted must exist.
		for _, bind := range hostConfig.Binds {
			splitBind := strings.Split(bind, ":")
			source := splitBind[0]

			// refuse to bind mount "/" to the container
			if source == "/" {
				return job.Errorf("Invalid bind mount '%s' : source can't be '/'", bind)
			}

			// ensure the source exists on the host
			_, err := os.Stat(source)
			if err != nil && os.IsNotExist(err) {
				err = os.MkdirAll(source, 0755)
				if err != nil {
					return job.Errorf("Could not create local directory '%s' for bind mount: %s!", source, err.Error())
				}
			}
		}
		// Register any links from the host config before starting the container
		if err := srv.daemon.RegisterLinks(container, hostConfig); err != nil {
			return job.Error(err)
		}
		container.SetHostConfig(hostConfig)
		container.ToDisk()
	}
	if err := container.Start(); err != nil {
		return job.Errorf("Cannot start container %s: %s", name, err)
	}
	srv.LogEvent("start", container.ID, daemon.Repositories().ImageName(container.Image))

	return engine.StatusOK
}

func (srv *Server) ContainerStop(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}
	var (
		name = job.Args[0]
		t    = 10
	)
	if job.EnvExists("t") {
		t = job.GetenvInt("t")
	}
	if container := srv.daemon.Get(name); container != nil {
		if err := container.Stop(int(t)); err != nil {
			return job.Errorf("Cannot stop container %s: %s\n", name, err)
		}
		srv.LogEvent("stop", container.ID, srv.daemon.Repositories().ImageName(container.Image))
	} else {
		return job.Errorf("No such container: %s\n", name)
	}
	return engine.StatusOK
}

func (srv *Server) ContainerWait(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s", job.Name)
	}
	name := job.Args[0]
	if container := srv.daemon.Get(name); container != nil {
		status := container.Wait()
		job.Printf("%d\n", status)
		return engine.StatusOK
	}
	return job.Errorf("%s: no such container: %s", job.Name, name)
}

func (srv *Server) ContainerResize(job *engine.Job) engine.Status {
	if len(job.Args) != 3 {
		return job.Errorf("Not enough arguments. Usage: %s CONTAINER HEIGHT WIDTH\n", job.Name)
	}
	name := job.Args[0]
	height, err := strconv.Atoi(job.Args[1])
	if err != nil {
		return job.Error(err)
	}
	width, err := strconv.Atoi(job.Args[2])
	if err != nil {
		return job.Error(err)
	}
	if container := srv.daemon.Get(name); container != nil {
		if err := container.Resize(height, width); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
	}
	return job.Errorf("No such container: %s", name)
}

func (srv *Server) ContainerLogs(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}

	var (
		name   = job.Args[0]
		stdout = job.GetenvBool("stdout")
		stderr = job.GetenvBool("stderr")
		follow = job.GetenvBool("follow")
		times  = job.GetenvBool("timestamps")
		format string
	)
	if !(stdout || stderr) {
		return job.Errorf("You must choose at least one stream")
	}
	if times {
		format = time.StampMilli
	}
	container := srv.daemon.Get(name)
	if container == nil {
		return job.Errorf("No such container: %s", name)
	}
	cLog, err := container.ReadLog("json")
	if err != nil && os.IsNotExist(err) {
		// Legacy logs
		utils.Debugf("Old logs format")
		if stdout {
			cLog, err := container.ReadLog("stdout")
			if err != nil {
				utils.Errorf("Error reading logs (stdout): %s", err)
			} else if _, err := io.Copy(job.Stdout, cLog); err != nil {
				utils.Errorf("Error streaming logs (stdout): %s", err)
			}
		}
		if stderr {
			cLog, err := container.ReadLog("stderr")
			if err != nil {
				utils.Errorf("Error reading logs (stderr): %s", err)
			} else if _, err := io.Copy(job.Stderr, cLog); err != nil {
				utils.Errorf("Error streaming logs (stderr): %s", err)
			}
		}
	} else if err != nil {
		utils.Errorf("Error reading logs (json): %s", err)
	} else {
		dec := json.NewDecoder(cLog)
		for {
			l := &utils.JSONLog{}

			if err := dec.Decode(l); err == io.EOF {
				break
			} else if err != nil {
				utils.Errorf("Error streaming logs: %s", err)
				break
			}
			logLine := l.Log
			if times {
				logLine = fmt.Sprintf("[%s] %s", l.Created.Format(format), logLine)
			}
			if l.Stream == "stdout" && stdout {
				fmt.Fprintf(job.Stdout, "%s", logLine)
			}
			if l.Stream == "stderr" && stderr {
				fmt.Fprintf(job.Stderr, "%s", logLine)
			}
		}
	}
	if follow {
		errors := make(chan error, 2)
		if stdout {
			stdoutPipe := container.StdoutLogPipe()
			go func() {
				errors <- utils.WriteLog(stdoutPipe, job.Stdout, format)
			}()
		}
		if stderr {
			stderrPipe := container.StderrLogPipe()
			go func() {
				errors <- utils.WriteLog(stderrPipe, job.Stderr, format)
			}()
		}
		err := <-errors
		if err != nil {
			utils.Errorf("%s", err)
		}
	}
	return engine.StatusOK
}

func (srv *Server) ContainerAttach(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}

	var (
		name   = job.Args[0]
		logs   = job.GetenvBool("logs")
		stream = job.GetenvBool("stream")
		stdin  = job.GetenvBool("stdin")
		stdout = job.GetenvBool("stdout")
		stderr = job.GetenvBool("stderr")
	)

	container := srv.daemon.Get(name)
	if container == nil {
		return job.Errorf("No such container: %s", name)
	}

	//logs
	if logs {
		cLog, err := container.ReadLog("json")
		if err != nil && os.IsNotExist(err) {
			// Legacy logs
			utils.Debugf("Old logs format")
			if stdout {
				cLog, err := container.ReadLog("stdout")
				if err != nil {
					utils.Errorf("Error reading logs (stdout): %s", err)
				} else if _, err := io.Copy(job.Stdout, cLog); err != nil {
					utils.Errorf("Error streaming logs (stdout): %s", err)
				}
			}
			if stderr {
				cLog, err := container.ReadLog("stderr")
				if err != nil {
					utils.Errorf("Error reading logs (stderr): %s", err)
				} else if _, err := io.Copy(job.Stderr, cLog); err != nil {
					utils.Errorf("Error streaming logs (stderr): %s", err)
				}
			}
		} else if err != nil {
			utils.Errorf("Error reading logs (json): %s", err)
		} else {
			dec := json.NewDecoder(cLog)
			for {
				l := &utils.JSONLog{}

				if err := dec.Decode(l); err == io.EOF {
					break
				} else if err != nil {
					utils.Errorf("Error streaming logs: %s", err)
					break
				}
				if l.Stream == "stdout" && stdout {
					fmt.Fprintf(job.Stdout, "%s", l.Log)
				}
				if l.Stream == "stderr" && stderr {
					fmt.Fprintf(job.Stderr, "%s", l.Log)
				}
			}
		}
	}

	//stream
	if stream {
		var (
			cStdin           io.ReadCloser
			cStdout, cStderr io.Writer
			cStdinCloser     io.Closer
		)

		if stdin {
			r, w := io.Pipe()
			go func() {
				defer w.Close()
				defer utils.Debugf("Closing buffered stdin pipe")
				io.Copy(w, job.Stdin)
			}()
			cStdin = r
			cStdinCloser = job.Stdin
		}
		if stdout {
			cStdout = job.Stdout
		}
		if stderr {
			cStderr = job.Stderr
		}

		<-srv.daemon.Attach(container, cStdin, cStdinCloser, cStdout, cStderr)

		// If we are in stdinonce mode, wait for the process to end
		// otherwise, simply return
		if container.Config.StdinOnce && !container.Config.Tty {
			container.Wait()
		}
	}
	return engine.StatusOK
}

func (srv *Server) ContainerCopy(job *engine.Job) engine.Status {
	if len(job.Args) != 2 {
		return job.Errorf("Usage: %s CONTAINER RESOURCE\n", job.Name)
	}

	var (
		name     = job.Args[0]
		resource = job.Args[1]
	)

	if container := srv.daemon.Get(name); container != nil {

		data, err := container.Copy(resource)
		if err != nil {
			return job.Error(err)
		}
		defer data.Close()

		if _, err := io.Copy(job.Stdout, data); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
	}
	return job.Errorf("No such container: %s", name)
}

func NewServer(eng *engine.Engine, config *daemonconfig.Config) (*Server, error) {
	daemon, err := daemon.NewDaemon(config, eng)
	if err != nil {
		return nil, err
	}
	srv := &Server{
		Eng:         eng,
		daemon:      daemon,
		pullingPool: make(map[string]chan struct{}),
		pushingPool: make(map[string]chan struct{}),
		events:      make([]utils.JSONMessage, 0, 64), //only keeps the 64 last events
		listeners:   make(map[int64]chan utils.JSONMessage),
	}
	daemon.SetServer(srv)
	return srv, nil
}

func (srv *Server) LogEvent(action, id, from string) *utils.JSONMessage {
	now := time.Now().UTC().Unix()
	jm := utils.JSONMessage{Status: action, ID: id, From: from, Time: now}
	srv.AddEvent(jm)
	for _, c := range srv.listeners {
		select { // non blocking channel
		case c <- jm:
		default:
		}
	}
	return &jm
}

func (srv *Server) AddEvent(jm utils.JSONMessage) {
	srv.Lock()
	defer srv.Unlock()
	srv.events = append(srv.events, jm)
}

func (srv *Server) GetEvents() []utils.JSONMessage {
	srv.RLock()
	defer srv.RUnlock()
	return srv.events
}

func (srv *Server) SetRunning(status bool) {
	srv.Lock()
	defer srv.Unlock()

	srv.running = status
}

func (srv *Server) IsRunning() bool {
	srv.RLock()
	defer srv.RUnlock()
	return srv.running
}

func (srv *Server) Close() error {
	if srv == nil {
		return nil
	}
	srv.SetRunning(false)
	done := make(chan struct{})
	go func() {
		srv.tasks.Wait()
		close(done)
	}()
	select {
	// Waiting server jobs for 15 seconds, shutdown immediately after that time
	case <-time.After(time.Second * 15):
	case <-done:
	}
	if srv.daemon == nil {
		return nil
	}
	return srv.daemon.Close()
}

type Server struct {
	sync.RWMutex
	daemon      *daemon.Daemon
	pullingPool map[string]chan struct{}
	pushingPool map[string]chan struct{}
	events      []utils.JSONMessage
	listeners   map[int64]chan utils.JSONMessage
	Eng         *engine.Engine
	running     bool
	tasks       sync.WaitGroup
}
