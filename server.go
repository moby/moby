package docker

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/auth"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/graphdb"
	"github.com/dotcloud/docker/registry"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

func (srv *Server) Close() error {
	return srv.runtime.Close()
}

func init() {
	engine.Register("initapi", jobInitApi)
}

// jobInitApi runs the remote api server `srv` as a daemon,
// Only one api server can run at the same time - this is enforced by a pidfile.
// The signals SIGINT and SIGTERM are intercepted for cleanup.
func jobInitApi(job *engine.Job) engine.Status {
	job.Logf("Creating server")
	// FIXME: ImportEnv deprecates ConfigFromJob
	srv, err := NewServer(job.Eng, ConfigFromJob(job))
	if err != nil {
		job.Error(err)
		return engine.StatusErr
	}
	if srv.runtime.config.Pidfile != "" {
		job.Logf("Creating pidfile")
		if err := utils.CreatePidFile(srv.runtime.config.Pidfile); err != nil {
			// FIXME: do we need fatal here instead of returning a job error?
			log.Fatal(err)
		}
	}
	job.Logf("Setting up signal traps")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Signal(syscall.SIGTERM))
	go func() {
		sig := <-c
		log.Printf("Received signal '%v', exiting\n", sig)
		utils.RemovePidFile(srv.runtime.config.Pidfile)
		srv.Close()
		os.Exit(0)
	}()
	job.Eng.Hack_SetGlobalVar("httpapi.server", srv)
	job.Eng.Hack_SetGlobalVar("httpapi.runtime", srv.runtime)
	// https://github.com/dotcloud/docker/issues/2768
	if srv.runtime.networkManager.bridgeNetwork != nil {
		job.Eng.Hack_SetGlobalVar("httpapi.bridgeIP", srv.runtime.networkManager.bridgeNetwork.IP)
	}
	if err := job.Eng.Register("create", srv.ContainerCreate); err != nil {
		job.Error(err)
		return engine.StatusErr
	}
	if err := job.Eng.Register("start", srv.ContainerStart); err != nil {
		job.Error(err)
		return engine.StatusErr
	}
	if err := job.Eng.Register("serveapi", srv.ListenAndServe); err != nil {
		job.Error(err)
		return engine.StatusErr
	}
	return engine.StatusOK
}

func (srv *Server) ListenAndServe(job *engine.Job) engine.Status {
	protoAddrs := job.Args
	chErrors := make(chan error, len(protoAddrs))
	for _, protoAddr := range protoAddrs {
		protoAddrParts := strings.SplitN(protoAddr, "://", 2)
		switch protoAddrParts[0] {
		case "unix":
			if err := syscall.Unlink(protoAddrParts[1]); err != nil && !os.IsNotExist(err) {
				log.Fatal(err)
			}
		case "tcp":
			if !strings.HasPrefix(protoAddrParts[1], "127.0.0.1") {
				log.Println("/!\\ DON'T BIND ON ANOTHER IP ADDRESS THAN 127.0.0.1 IF YOU DON'T KNOW WHAT YOU'RE DOING /!\\")
			}
		default:
			job.Errorf("Invalid protocol format.")
			return engine.StatusErr
		}
		go func() {
			// FIXME: merge Server.ListenAndServe with ListenAndServe
			chErrors <- ListenAndServe(protoAddrParts[0], protoAddrParts[1], srv, job.GetenvBool("Logging"))
		}()
	}
	for i := 0; i < len(protoAddrs); i += 1 {
		err := <-chErrors
		if err != nil {
			job.Error(err)
			return engine.StatusErr
		}
	}
	return engine.StatusOK
}

func (srv *Server) DockerVersion() APIVersion {
	return APIVersion{
		Version:   VERSION,
		GitCommit: GITCOMMIT,
		GoVersion: runtime.Version(),
	}
}

// simpleVersionInfo is a simple implementation of
// the interface VersionInfo, which is used
// to provide version information for some product,
// component, etc. It stores the product name and the version
// in string and returns them on calls to Name() and Version().
type simpleVersionInfo struct {
	name    string
	version string
}

func (v *simpleVersionInfo) Name() string {
	return v.name
}

func (v *simpleVersionInfo) Version() string {
	return v.version
}

// versionCheckers() returns version informations of:
// docker, go, git-commit (of the docker) and the host's kernel.
//
// Such information will be used on call to NewRegistry().
func (srv *Server) versionInfos() []utils.VersionInfo {
	v := srv.DockerVersion()
	ret := append(make([]utils.VersionInfo, 0, 4), &simpleVersionInfo{"docker", v.Version})

	if len(v.GoVersion) > 0 {
		ret = append(ret, &simpleVersionInfo{"go", v.GoVersion})
	}
	if len(v.GitCommit) > 0 {
		ret = append(ret, &simpleVersionInfo{"git-commit", v.GitCommit})
	}
	if kernelVersion, err := utils.GetKernelVersion(); err == nil {
		ret = append(ret, &simpleVersionInfo{"kernel", kernelVersion.String()})
	}

	return ret
}

// ContainerKill send signal to the container
// If no signal is given (sig 0), then Kill with SIGKILL and wait
// for the container to exit.
// If a signal is given, then just send it to the container and return.
func (srv *Server) ContainerKill(name string, sig int) error {
	if container := srv.runtime.Get(name); container != nil {
		// If no signal is passed, perform regular Kill (SIGKILL + wait())
		if sig == 0 {
			if err := container.Kill(); err != nil {
				return fmt.Errorf("Cannot kill container %s: %s", name, err)
			}
			srv.LogEvent("kill", container.ID, srv.runtime.repositories.ImageName(container.Image))
		} else {
			// Otherwise, just send the requested signal
			if err := container.kill(sig); err != nil {
				return fmt.Errorf("Cannot kill container %s: %s", name, err)
			}
			// FIXME: Add event for signals
		}
	} else {
		return fmt.Errorf("No such container: %s", name)
	}
	return nil
}

func (srv *Server) ContainerExport(name string, out io.Writer) error {
	if container := srv.runtime.Get(name); container != nil {

		data, err := container.Export()
		if err != nil {
			return err
		}

		// Stream the entire contents of the container (basically a volatile snapshot)
		if _, err := io.Copy(out, data); err != nil {
			return err
		}
		srv.LogEvent("export", container.ID, srv.runtime.repositories.ImageName(container.Image))
		return nil
	}
	return fmt.Errorf("No such container: %s", name)
}

// ImageExport exports all images with the given tag. All versions
// containing the same tag are exported. The resulting output is an
// uncompressed tar ball.
// name is the set of tags to export.
// out is the writer where the images are written to.
func (srv *Server) ImageExport(name string, out io.Writer) error {
	// get image json
	tempdir, err := ioutil.TempDir("", "docker-export-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempdir)

	utils.Debugf("Serializing %s", name)

	rootRepo, err := srv.runtime.repositories.Get(name)
	if err != nil {
		return err
	}
	if rootRepo != nil {
		for _, id := range rootRepo {
			image, err := srv.ImageInspect(id)
			if err != nil {
				return err
			}

			if err := srv.exportImage(image, tempdir); err != nil {
				return err
			}
		}

		// write repositories
		rootRepoMap := map[string]Repository{}
		rootRepoMap[name] = rootRepo
		rootRepoJson, _ := json.Marshal(rootRepoMap)

		if err := ioutil.WriteFile(path.Join(tempdir, "repositories"), rootRepoJson, os.ModeAppend); err != nil {
			return err
		}
	} else {
		image, err := srv.ImageInspect(name)
		if err != nil {
			return err
		}
		if err := srv.exportImage(image, tempdir); err != nil {
			return err
		}
	}

	fs, err := archive.Tar(tempdir, archive.Uncompressed)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, fs); err != nil {
		return err
	}
	return nil
}

func (srv *Server) exportImage(image *Image, tempdir string) error {
	for i := image; i != nil; {
		// temporary directory
		tmpImageDir := path.Join(tempdir, i.ID)
		if err := os.Mkdir(tmpImageDir, os.ModeDir); err != nil {
			return err
		}

		var version = "1.0"
		var versionBuf = []byte(version)

		if err := ioutil.WriteFile(path.Join(tmpImageDir, "VERSION"), versionBuf, os.ModeAppend); err != nil {
			return err
		}

		// serialize json
		b, err := json.Marshal(i)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(path.Join(tmpImageDir, "json"), b, os.ModeAppend); err != nil {
			return err
		}

		// serialize filesystem
		fs, err := i.TarLayer()
		if err != nil {
			return err
		}

		fsTar, err := os.Create(path.Join(tmpImageDir, "layer.tar"))
		if err != nil {
			return err
		}
		if _, err = io.Copy(fsTar, fs); err != nil {
			return err
		}
		fsTar.Close()

		// find parent
		if i.Parent != "" {
			i, err = srv.ImageInspect(i.Parent)
			if err != nil {
				return err
			}
		} else {
			i = nil
		}
	}
	return nil
}

// Loads a set of images into the repository. This is the complementary of ImageExport.
// The input stream is an uncompressed tar ball containing images and metadata.
func (srv *Server) ImageLoad(in io.Reader) error {
	tmpImageDir, err := ioutil.TempDir("", "docker-import-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpImageDir)

	var (
		repoTarFile = path.Join(tmpImageDir, "repo.tar")
		repoDir     = path.Join(tmpImageDir, "repo")
	)

	tarFile, err := os.Create(repoTarFile)
	if err != nil {
		return err
	}
	if _, err := io.Copy(tarFile, in); err != nil {
		return err
	}
	tarFile.Close()

	repoFile, err := os.Open(repoTarFile)
	if err != nil {
		return err
	}
	if err := os.Mkdir(repoDir, os.ModeDir); err != nil {
		return err
	}
	if err := archive.Untar(repoFile, repoDir, nil); err != nil {
		return err
	}

	dirs, err := ioutil.ReadDir(repoDir)
	if err != nil {
		return err
	}

	for _, d := range dirs {
		if d.IsDir() {
			if err := srv.recursiveLoad(d.Name(), tmpImageDir); err != nil {
				return err
			}
		}
	}

	repositoriesJson, err := ioutil.ReadFile(path.Join(tmpImageDir, "repo", "repositories"))
	if err == nil {
		repositories := map[string]Repository{}
		if err := json.Unmarshal(repositoriesJson, &repositories); err != nil {
			return err
		}

		for imageName, tagMap := range repositories {
			for tag, address := range tagMap {
				if err := srv.runtime.repositories.Set(imageName, tag, address, true); err != nil {
					return err
				}
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (srv *Server) recursiveLoad(address, tmpImageDir string) error {
	if _, err := srv.ImageInspect(address); err != nil {
		utils.Debugf("Loading %s", address)

		imageJson, err := ioutil.ReadFile(path.Join(tmpImageDir, "repo", address, "json"))
		if err != nil {
			return err
			utils.Debugf("Error reading json", err)
		}

		layer, err := os.Open(path.Join(tmpImageDir, "repo", address, "layer.tar"))
		if err != nil {
			utils.Debugf("Error reading embedded tar", err)
			return err
		}
		img, err := NewImgJSON(imageJson)
		if err != nil {
			utils.Debugf("Error unmarshalling json", err)
			return err
		}
		if img.Parent != "" {
			if !srv.runtime.graph.Exists(img.Parent) {
				if err := srv.recursiveLoad(img.Parent, tmpImageDir); err != nil {
					return err
				}
			}
		}
		if err := srv.runtime.graph.Register(imageJson, layer, img); err != nil {
			return err
		}
	}
	utils.Debugf("Completed processing %s", address)

	return nil
}

func (srv *Server) ImagesSearch(term string) ([]registry.SearchResult, error) {
	r, err := registry.NewRegistry(srv.runtime.config.Root, nil, srv.HTTPRequestFactory(nil))
	if err != nil {
		return nil, err
	}
	results, err := r.SearchRepositories(term)
	if err != nil {
		return nil, err
	}
	return results.Results, nil
}

func (srv *Server) ImageInsert(name, url, path string, out io.Writer, sf *utils.StreamFormatter) error {
	out = utils.NewWriteFlusher(out)
	img, err := srv.runtime.repositories.LookupImage(name)
	if err != nil {
		return err
	}

	file, err := utils.Download(url, out)
	if err != nil {
		return err
	}
	defer file.Body.Close()

	config, _, _, err := ParseRun([]string{img.ID, "echo", "insert", url, path}, srv.runtime.capabilities)
	if err != nil {
		return err
	}

	c, _, err := srv.runtime.Create(config, "")
	if err != nil {
		return err
	}

	if err := c.Inject(utils.ProgressReader(file.Body, int(file.ContentLength), out, sf.FormatProgress("", "Downloading", "%8v/%v (%v)"), sf, false), path); err != nil {
		return err
	}
	// FIXME: Handle custom repo, tag comment, author
	img, err = srv.runtime.Commit(c, "", "", img.Comment, img.Author, nil)
	if err != nil {
		return err
	}
	out.Write(sf.FormatStatus(img.ID, ""))
	return nil
}

func (srv *Server) ImagesViz(out io.Writer) error {
	images, _ := srv.runtime.graph.Map()
	if images == nil {
		return nil
	}
	out.Write([]byte("digraph docker {\n"))

	var (
		parentImage *Image
		err         error
	)
	for _, image := range images {
		parentImage, err = image.GetParent()
		if err != nil {
			return fmt.Errorf("Error while getting parent image: %v", err)
		}
		if parentImage != nil {
			out.Write([]byte(" \"" + parentImage.ID + "\" -> \"" + image.ID + "\"\n"))
		} else {
			out.Write([]byte(" base -> \"" + image.ID + "\" [style=invis]\n"))
		}
	}

	reporefs := make(map[string][]string)

	for name, repository := range srv.runtime.repositories.Repositories {
		for tag, id := range repository {
			reporefs[utils.TruncateID(id)] = append(reporefs[utils.TruncateID(id)], fmt.Sprintf("%s:%s", name, tag))
		}
	}

	for id, repos := range reporefs {
		out.Write([]byte(" \"" + id + "\" [label=\"" + id + "\\n" + strings.Join(repos, "\\n") + "\",shape=box,fillcolor=\"paleturquoise\",style=\"filled,rounded\"];\n"))
	}
	out.Write([]byte(" base [style=invisible]\n}\n"))
	return nil
}

func (srv *Server) Images(all bool, filter string) ([]APIImages, error) {
	var (
		allImages map[string]*Image
		err       error
	)
	if all {
		allImages, err = srv.runtime.graph.Map()
	} else {
		allImages, err = srv.runtime.graph.Heads()
	}
	if err != nil {
		return nil, err
	}
	lookup := make(map[string]APIImages)
	for name, repository := range srv.runtime.repositories.Repositories {
		if filter != "" {
			if match, _ := path.Match(filter, name); !match {
				continue
			}
		}
		for tag, id := range repository {
			image, err := srv.runtime.graph.Get(id)
			if err != nil {
				log.Printf("Warning: couldn't load %s from %s/%s: %s", id, name, tag, err)
				continue
			}

			if out, exists := lookup[id]; exists {
				out.RepoTags = append(out.RepoTags, fmt.Sprintf("%s:%s", name, tag))

				lookup[id] = out
			} else {
				var out APIImages

				delete(allImages, id)

				out.ParentId = image.Parent
				out.RepoTags = []string{fmt.Sprintf("%s:%s", name, tag)}
				out.ID = image.ID
				out.Created = image.Created.Unix()
				out.Size = image.Size
				out.VirtualSize = image.getParentsSize(0) + image.Size

				lookup[id] = out
			}

		}
	}

	outs := make([]APIImages, 0, len(lookup))
	for _, value := range lookup {
		outs = append(outs, value)
	}

	// Display images which aren't part of a repository/tag
	if filter == "" {
		for _, image := range allImages {
			var out APIImages
			out.ID = image.ID
			out.ParentId = image.Parent
			out.RepoTags = []string{"<none>:<none>"}
			out.Created = image.Created.Unix()
			out.Size = image.Size
			out.VirtualSize = image.getParentsSize(0) + image.Size
			outs = append(outs, out)
		}
	}

	sortImagesByCreationAndTag(outs)
	return outs, nil
}

func (srv *Server) DockerInfo() *APIInfo {
	images, _ := srv.runtime.graph.Map()
	var imgcount int
	if images == nil {
		imgcount = 0
	} else {
		imgcount = len(images)
	}
	lxcVersion := ""
	if output, err := exec.Command("lxc-version").CombinedOutput(); err == nil {
		outputStr := string(output)
		if len(strings.SplitN(outputStr, ":", 2)) == 2 {
			lxcVersion = strings.TrimSpace(strings.SplitN(string(output), ":", 2)[1])
		}
	}
	kernelVersion := "<unknown>"
	if kv, err := utils.GetKernelVersion(); err == nil {
		kernelVersion = kv.String()
	}

	return &APIInfo{
		Containers:         len(srv.runtime.List()),
		Images:             imgcount,
		Driver:             srv.runtime.driver.String(),
		DriverStatus:       srv.runtime.driver.Status(),
		MemoryLimit:        srv.runtime.capabilities.MemoryLimit,
		SwapLimit:          srv.runtime.capabilities.SwapLimit,
		IPv4Forwarding:     !srv.runtime.capabilities.IPv4ForwardingDisabled,
		Debug:              os.Getenv("DEBUG") != "",
		NFd:                utils.GetTotalUsedFds(),
		NGoroutines:        runtime.NumGoroutine(),
		LXCVersion:         lxcVersion,
		NEventsListener:    len(srv.events),
		KernelVersion:      kernelVersion,
		IndexServerAddress: auth.IndexServerAddress(),
	}
}

func (srv *Server) ImageHistory(name string) ([]APIHistory, error) {
	image, err := srv.runtime.repositories.LookupImage(name)
	if err != nil {
		return nil, err
	}

	lookupMap := make(map[string][]string)
	for name, repository := range srv.runtime.repositories.Repositories {
		for tag, id := range repository {
			// If the ID already has a reverse lookup, do not update it unless for "latest"
			if _, exists := lookupMap[id]; !exists {
				lookupMap[id] = []string{}
			}
			lookupMap[id] = append(lookupMap[id], name+":"+tag)
		}
	}

	outs := []APIHistory{} //produce [] when empty instead of 'null'
	err = image.WalkHistory(func(img *Image) error {
		var out APIHistory
		out.ID = img.ID
		out.Created = img.Created.Unix()
		out.CreatedBy = strings.Join(img.ContainerConfig.Cmd, " ")
		out.Tags = lookupMap[img.ID]
		out.Size = img.Size
		outs = append(outs, out)
		return nil
	})
	return outs, nil

}

func (srv *Server) ContainerTop(name, psArgs string) (*APITop, error) {
	if container := srv.runtime.Get(name); container != nil {
		output, err := exec.Command("lxc-ps", "--name", container.ID, "--", psArgs).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("lxc-ps: %s (%s)", err, output)
		}
		procs := APITop{}
		for i, line := range strings.Split(string(output), "\n") {
			if len(line) == 0 {
				continue
			}
			words := []string{}
			scanner := bufio.NewScanner(strings.NewReader(line))
			scanner.Split(bufio.ScanWords)
			if !scanner.Scan() {
				return nil, fmt.Errorf("Wrong output using lxc-ps")
			}
			// no scanner.Text because we skip container id
			for scanner.Scan() {
				if i != 0 && len(words) == len(procs.Titles) {
					words[len(words)-1] = fmt.Sprintf("%s %s", words[len(words)-1], scanner.Text())
				} else {
					words = append(words, scanner.Text())
				}
			}
			if i == 0 {
				procs.Titles = words
			} else {
				procs.Processes = append(procs.Processes, words)
			}
		}
		return &procs, nil

	}
	return nil, fmt.Errorf("No such container: %s", name)
}

func (srv *Server) ContainerChanges(name string) ([]archive.Change, error) {
	if container := srv.runtime.Get(name); container != nil {
		return container.Changes()
	}
	return nil, fmt.Errorf("No such container: %s", name)
}

func (srv *Server) Containers(all, size bool, n int, since, before string) []APIContainers {
	var foundBefore bool
	var displayed int
	out := []APIContainers{}

	names := map[string][]string{}
	srv.runtime.containerGraph.Walk("/", func(p string, e *graphdb.Entity) error {
		names[e.ID()] = append(names[e.ID()], p)
		return nil
	}, -1)

	for _, container := range srv.runtime.List() {
		if !container.State.IsRunning() && !all && n == -1 && since == "" && before == "" {
			continue
		}
		if before != "" && !foundBefore {
			if container.ID == before || utils.TruncateID(container.ID) == before {
				foundBefore = true
			}
			continue
		}
		if displayed == n {
			break
		}
		if container.ID == since || utils.TruncateID(container.ID) == since {
			break
		}
		displayed++
		c := createAPIContainer(names[container.ID], container, size, srv.runtime)
		out = append(out, c)
	}
	return out
}

func createAPIContainer(names []string, container *Container, size bool, runtime *Runtime) APIContainers {
	c := APIContainers{
		ID: container.ID,
	}
	c.Names = names
	c.Image = runtime.repositories.ImageName(container.Image)
	c.Command = fmt.Sprintf("%s %s", container.Path, strings.Join(container.Args, " "))
	c.Created = container.Created.Unix()
	c.Status = container.State.String()
	c.Ports = container.NetworkSettings.PortMappingAPI()
	if size {
		c.SizeRw, c.SizeRootFs = container.GetSize()
	}
	return c
}
func (srv *Server) ContainerCommit(name, repo, tag, author, comment string, config *Config) (string, error) {
	container := srv.runtime.Get(name)
	if container == nil {
		return "", fmt.Errorf("No such container: %s", name)
	}
	img, err := srv.runtime.Commit(container, repo, tag, comment, author, config)
	if err != nil {
		return "", err
	}
	return img.ID, err
}

// FIXME: this should be called ImageTag
func (srv *Server) ContainerTag(name, repo, tag string, force bool) error {
	if err := srv.runtime.repositories.Set(repo, tag, name, force); err != nil {
		return err
	}
	return nil
}

func (srv *Server) pullImage(r *registry.Registry, out io.Writer, imgID, endpoint string, token []string, sf *utils.StreamFormatter) error {
	history, err := r.GetRemoteHistory(imgID, endpoint, token)
	if err != nil {
		return err
	}
	out.Write(sf.FormatProgress(utils.TruncateID(imgID), "Pulling", "dependent layers"))
	// FIXME: Try to stream the images?
	// FIXME: Launch the getRemoteImage() in goroutines

	for i := len(history) - 1; i >= 0; i-- {
		id := history[i]

		// ensure no two downloads of the same layer happen at the same time
		if c, err := srv.poolAdd("pull", "layer:"+id); err != nil {
			utils.Errorf("Image (id: %s) pull is already running, skipping: %v", id, err)
			<-c
		}
		defer srv.poolRemove("pull", "layer:"+id)

		if !srv.runtime.graph.Exists(id) {
			out.Write(sf.FormatProgress(utils.TruncateID(id), "Pulling", "metadata"))
			imgJSON, imgSize, err := r.GetRemoteImageJSON(id, endpoint, token)
			if err != nil {
				out.Write(sf.FormatProgress(utils.TruncateID(id), "Error", "pulling dependent layers"))
				// FIXME: Keep going in case of error?
				return err
			}
			img, err := NewImgJSON(imgJSON)
			if err != nil {
				out.Write(sf.FormatProgress(utils.TruncateID(id), "Error", "pulling dependent layers"))
				return fmt.Errorf("Failed to parse json: %s", err)
			}

			// Get the layer
			out.Write(sf.FormatProgress(utils.TruncateID(id), "Pulling", "fs layer"))
			layer, err := r.GetRemoteImageLayer(img.ID, endpoint, token)
			if err != nil {
				out.Write(sf.FormatProgress(utils.TruncateID(id), "Error", "pulling dependent layers"))
				return err
			}
			defer layer.Close()
			if err := srv.runtime.graph.Register(imgJSON, utils.ProgressReader(layer, imgSize, out, sf.FormatProgress(utils.TruncateID(id), "Downloading", "%8v/%v (%v)"), sf, false), img); err != nil {
				out.Write(sf.FormatProgress(utils.TruncateID(id), "Error", "downloading dependent layers"))
				return err
			}
		}
		out.Write(sf.FormatProgress(utils.TruncateID(id), "Download", "complete"))

	}
	return nil
}

func (srv *Server) pullRepository(r *registry.Registry, out io.Writer, localName, remoteName, askedTag, indexEp string, sf *utils.StreamFormatter, parallel bool) error {
	out.Write(sf.FormatStatus("", "Pulling repository %s", localName))

	repoData, err := r.GetRepositoryData(indexEp, remoteName)
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
			if _, err := srv.poolAdd("pull", "img:"+img.ID); err != nil {
				utils.Errorf("Image (id: %s) pull is already running, skipping: %v", img.ID, err)
				if parallel {
					errors <- nil
				}
				return
			}
			defer srv.poolRemove("pull", "img:"+img.ID)

			out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Pulling", fmt.Sprintf("image (%s) from %s", img.Tag, localName)))
			success := false
			var lastErr error
			for _, ep := range repoData.Endpoints {
				out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Pulling", fmt.Sprintf("image (%s) from %s, endpoint: %s", img.Tag, localName, ep)))
				if err := srv.pullImage(r, out, img.ID, ep, repoData.Tokens, sf); err != nil {
					// Its not ideal that only the last error  is returned, it would be better to concatenate the errors.
					// As the error is also given to the output stream the user will see the error.
					lastErr = err
					out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Error pulling", fmt.Sprintf("image (%s) from %s, endpoint: %s, %s", img.Tag, localName, ep, err)))
					continue
				}
				success = true
				break
			}
			if !success {
				out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Error pulling", fmt.Sprintf("image (%s) from %s, %s", img.Tag, localName, lastErr)))
				if parallel {
					errors <- fmt.Errorf("Could not find repository on any of the indexed registries.")
					return
				}
			}
			out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Download", "complete"))

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
		if err := srv.runtime.repositories.Set(localName, tag, id, true); err != nil {
			return err
		}
	}
	if err := srv.runtime.repositories.Save(); err != nil {
		return err
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

func (srv *Server) ImagePull(localName string, tag string, out io.Writer, sf *utils.StreamFormatter, authConfig *auth.AuthConfig, metaHeaders map[string][]string, parallel bool) error {
	r, err := registry.NewRegistry(srv.runtime.config.Root, authConfig, srv.HTTPRequestFactory(metaHeaders))
	if err != nil {
		return err
	}

	out = utils.NewWriteFlusher(out)

	c, err := srv.poolAdd("pull", localName+":"+tag)
	if err != nil {
		if c != nil {
			// Another pull of the same repository is already taking place; just wait for it to finish
			out.Write(sf.FormatStatus("", "Repository %s already being pulled by another client. Waiting.", localName))
			<-c
			return nil
		}
		return err
	}
	defer srv.poolRemove("pull", localName+":"+tag)

	// Resolve the Repository name from fqn to endpoint + name
	endpoint, remoteName, err := registry.ResolveRepositoryName(localName)
	if err != nil {
		return err
	}

	if endpoint == auth.IndexServerAddress() {
		// If pull "index.docker.io/foo/bar", it's stored locally under "foo/bar"
		localName = remoteName
	}

	if err = srv.pullRepository(r, out, localName, remoteName, tag, endpoint, sf, parallel); err != nil {
		return err
	}

	return nil
}

// Retrieve the all the images to be uploaded in the correct order
// Note: we can't use a map as it is not ordered
func (srv *Server) getImageList(localRepo map[string]string) ([][]*registry.ImgData, error) {
	imgList := map[string]*registry.ImgData{}
	depGraph := utils.NewDependencyGraph()

	for tag, id := range localRepo {
		img, err := srv.runtime.graph.Get(id)
		if err != nil {
			return nil, err
		}
		depGraph.NewNode(img.ID)
		img.WalkHistory(func(current *Image) error {
			imgList[current.ID] = &registry.ImgData{
				ID:  current.ID,
				Tag: tag,
			}
			parent, err := current.GetParent()
			if err != nil {
				return err
			}
			if parent == nil {
				return nil
			}
			depGraph.NewNode(parent.ID)
			depGraph.AddDependency(current.ID, parent.ID)
			return nil
		})
	}

	traversalMap, err := depGraph.GenerateTraversalMap()
	if err != nil {
		return nil, err
	}

	utils.Debugf("Traversal map: %v", traversalMap)
	result := [][]*registry.ImgData{}
	for _, round := range traversalMap {
		dataRound := []*registry.ImgData{}
		for _, imgID := range round {
			dataRound = append(dataRound, imgList[imgID])
		}
		result = append(result, dataRound)
	}
	return result, nil
}

func flatten(slc [][]*registry.ImgData) []*registry.ImgData {
	result := []*registry.ImgData{}
	for _, x := range slc {
		result = append(result, x...)
	}
	return result
}

func (srv *Server) pushRepository(r *registry.Registry, out io.Writer, localName, remoteName string, localRepo map[string]string, indexEp string, sf *utils.StreamFormatter) error {
	out = utils.NewWriteFlusher(out)
	imgList, err := srv.getImageList(localRepo)
	if err != nil {
		return err
	}
	flattenedImgList := flatten(imgList)
	out.Write(sf.FormatStatus("", "Sending image list"))

	var repoData *registry.RepositoryData
	repoData, err = r.PushImageJSONIndex(indexEp, remoteName, flattenedImgList, false, nil)
	if err != nil {
		return err
	}

	for _, ep := range repoData.Endpoints {
		out.Write(sf.FormatStatus("", "Pushing repository %s (%d tags)", localName, len(localRepo)))
		// This section can not be parallelized (each round depends on the previous one)
		for _, round := range imgList {
			// FIXME: This section can be parallelized
			for _, elem := range round {
				var pushTags func() error
				pushTags = func() error {
					out.Write(sf.FormatStatus("", "Pushing tags for rev [%s] on {%s}", elem.ID, ep+"repositories/"+remoteName+"/tags/"+elem.Tag))
					if err := r.PushRegistryTag(remoteName, elem.ID, elem.Tag, ep, repoData.Tokens); err != nil {
						return err
					}
					return nil
				}
				if _, exists := repoData.ImgList[elem.ID]; exists {
					if err := pushTags(); err != nil {
						return err
					}
					out.Write(sf.FormatStatus("", "Image %s already pushed, skipping", elem.ID))
					continue
				} else if r.LookupRemoteImage(elem.ID, ep, repoData.Tokens) {
					if err := pushTags(); err != nil {
						return err
					}
					out.Write(sf.FormatStatus("", "Image %s already pushed, skipping", elem.ID))
					continue
				}
				checksum, err := srv.pushImage(r, out, remoteName, elem.ID, ep, repoData.Tokens, sf)
				if err != nil {
					// FIXME: Continue on error?
					return err
				}
				elem.Checksum = checksum

				if err := pushTags(); err != nil {
					return err
				}
			}
		}
	}

	if _, err := r.PushImageJSONIndex(indexEp, remoteName, flattenedImgList, true, repoData.Endpoints); err != nil {
		return err
	}

	return nil
}

func (srv *Server) pushImage(r *registry.Registry, out io.Writer, remote, imgID, ep string, token []string, sf *utils.StreamFormatter) (checksum string, err error) {
	out = utils.NewWriteFlusher(out)
	jsonRaw, err := ioutil.ReadFile(path.Join(srv.runtime.graph.Root, imgID, "json"))
	if err != nil {
		return "", fmt.Errorf("Cannot retrieve the path for {%s}: %s", imgID, err)
	}
	out.Write(sf.FormatStatus("", "Pushing %s", imgID))

	imgData := &registry.ImgData{
		ID: imgID,
	}

	// Send the json
	if err := r.PushImageJSONRegistry(imgData, jsonRaw, ep, token); err != nil {
		if err == registry.ErrAlreadyExists {
			out.Write(sf.FormatStatus("", "Image %s already pushed, skipping", imgData.ID))
			return "", nil
		}
		return "", err
	}

	layerData, err := srv.runtime.graph.TempLayerArchive(imgID, archive.Uncompressed, sf, out)
	if err != nil {
		return "", fmt.Errorf("Failed to generate layer archive: %s", err)
	}
	defer os.RemoveAll(layerData.Name())

	// Send the layer
	checksum, err = r.PushImageLayerRegistry(imgData.ID, utils.ProgressReader(layerData, int(layerData.Size), out, sf.FormatProgress("", "Pushing", "%8v/%v (%v)"), sf, false), ep, token, jsonRaw)
	if err != nil {
		return "", err
	}
	imgData.Checksum = checksum

	out.Write(sf.FormatStatus("", ""))

	// Send the checksum
	if err := r.PushImageChecksumRegistry(imgData, ep, token); err != nil {
		return "", err
	}

	return imgData.Checksum, nil
}

// FIXME: Allow to interrupt current push when new push of same image is done.
func (srv *Server) ImagePush(localName string, out io.Writer, sf *utils.StreamFormatter, authConfig *auth.AuthConfig, metaHeaders map[string][]string) error {
	if _, err := srv.poolAdd("push", localName); err != nil {
		return err
	}
	defer srv.poolRemove("push", localName)

	// Resolve the Repository name from fqn to endpoint + name
	endpoint, remoteName, err := registry.ResolveRepositoryName(localName)
	if err != nil {
		return err
	}

	out = utils.NewWriteFlusher(out)
	img, err := srv.runtime.graph.Get(localName)
	r, err2 := registry.NewRegistry(srv.runtime.config.Root, authConfig, srv.HTTPRequestFactory(metaHeaders))
	if err2 != nil {
		return err2
	}

	if err != nil {
		reposLen := len(srv.runtime.repositories.Repositories[localName])
		out.Write(sf.FormatStatus("", "The push refers to a repository [%s] (len: %d)", localName, reposLen))
		// If it fails, try to get the repository
		if localRepo, exists := srv.runtime.repositories.Repositories[localName]; exists {
			if err := srv.pushRepository(r, out, localName, remoteName, localRepo, endpoint, sf); err != nil {
				return err
			}
			return nil
		}
		return err
	}

	var token []string
	out.Write(sf.FormatStatus("", "The push refers to an image: [%s]", localName))
	if _, err := srv.pushImage(r, out, remoteName, img.ID, endpoint, token, sf); err != nil {
		return err
	}
	return nil
}

func (srv *Server) ImageImport(src, repo, tag string, in io.Reader, out io.Writer, sf *utils.StreamFormatter) error {
	var archive io.Reader
	var resp *http.Response

	if src == "-" {
		archive = in
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
		out.Write(sf.FormatStatus("", "Downloading from %s", u))
		// Download with curl (pretty progress bar)
		// If curl is not available, fallback to http.Get()
		resp, err = utils.Download(u.String(), out)
		if err != nil {
			return err
		}
		archive = utils.ProgressReader(resp.Body, int(resp.ContentLength), out, sf.FormatProgress("", "Importing", "%8v/%v (%v)"), sf, true)
	}
	img, err := srv.runtime.graph.Create(archive, nil, "Imported from "+src, "", nil)
	if err != nil {
		return err
	}
	// Optionally register the image at REPO/TAG
	if repo != "" {
		if err := srv.runtime.repositories.Set(repo, tag, img.ID, true); err != nil {
			return err
		}
	}
	out.Write(sf.FormatStatus("", img.ID))
	return nil
}

func (srv *Server) ContainerCreate(job *engine.Job) engine.Status {
	var name string
	if len(job.Args) == 1 {
		name = job.Args[0]
	} else if len(job.Args) > 1 {
		job.Printf("Usage: %s", job.Name)
		return engine.StatusErr
	}
	var config Config
	if err := job.ExportEnv(&config); err != nil {
		job.Error(err)
		return engine.StatusErr
	}
	if config.Memory != 0 && config.Memory < 524288 {
		job.Errorf("Minimum memory limit allowed is 512k")
		return engine.StatusErr
	}
	if config.Memory > 0 && !srv.runtime.capabilities.MemoryLimit {
		config.Memory = 0
	}
	if config.Memory > 0 && !srv.runtime.capabilities.SwapLimit {
		config.MemorySwap = -1
	}
	container, buildWarnings, err := srv.runtime.Create(&config, name)
	if err != nil {
		if srv.runtime.graph.IsNotExist(err) {
			_, tag := utils.ParseRepositoryTag(config.Image)
			if tag == "" {
				tag = DEFAULTTAG
			}
			job.Errorf("No such image: %s (tag: %s)", config.Image, tag)
			return engine.StatusErr
		}
		job.Error(err)
		return engine.StatusErr
	}
	srv.LogEvent("create", container.ID, srv.runtime.repositories.ImageName(container.Image))
	// FIXME: this is necessary because runtime.Create might return a nil container
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

func (srv *Server) ContainerRestart(name string, t int) error {
	if container := srv.runtime.Get(name); container != nil {
		if err := container.Restart(t); err != nil {
			return fmt.Errorf("Cannot restart container %s: %s", name, err)
		}
		srv.LogEvent("restart", container.ID, srv.runtime.repositories.ImageName(container.Image))
	} else {
		return fmt.Errorf("No such container: %s", name)
	}
	return nil
}

func (srv *Server) ContainerDestroy(name string, removeVolume, removeLink bool) error {
	container := srv.runtime.Get(name)

	if removeLink {
		if container == nil {
			return fmt.Errorf("No such link: %s", name)
		}
		name, err := srv.runtime.getFullName(name)
		if err != nil {
			return err
		}
		parent, n := path.Split(name)
		if parent == "/" {
			return fmt.Errorf("Conflict, cannot remove the default name of the container")
		}
		pe := srv.runtime.containerGraph.Get(parent)
		if pe == nil {
			return fmt.Errorf("Cannot get parent %s for name %s", parent, name)
		}
		parentContainer := srv.runtime.Get(pe.ID())

		if parentContainer != nil && parentContainer.activeLinks != nil {
			if link, exists := parentContainer.activeLinks[n]; exists {
				link.Disable()
			} else {
				utils.Debugf("Could not find active link for %s", name)
			}
		}

		if err := srv.runtime.containerGraph.Delete(name); err != nil {
			return err
		}
		return nil
	}

	if container != nil {
		if container.State.IsRunning() {
			return fmt.Errorf("Impossible to remove a running container, please stop it first")
		}
		volumes := make(map[string]struct{})

		binds := make(map[string]struct{})

		for _, bind := range container.hostConfig.Binds {
			splitBind := strings.Split(bind, ":")
			source := splitBind[0]
			binds[source] = struct{}{}
		}

		// Store all the deleted containers volumes
		for _, volumeId := range container.Volumes {

			// Skip the volumes mounted from external
			if _, exists := binds[volumeId]; exists {
				continue
			}

			volumeId = strings.TrimSuffix(volumeId, "/layer")
			volumeId = filepath.Base(volumeId)
			volumes[volumeId] = struct{}{}
		}
		if err := srv.runtime.Destroy(container); err != nil {
			return fmt.Errorf("Cannot destroy container %s: %s", name, err)
		}
		srv.LogEvent("destroy", container.ID, srv.runtime.repositories.ImageName(container.Image))

		if removeVolume {
			// Retrieve all volumes from all remaining containers
			usedVolumes := make(map[string]*Container)
			for _, container := range srv.runtime.List() {
				for _, containerVolumeId := range container.Volumes {
					usedVolumes[containerVolumeId] = container
				}
			}

			for volumeId := range volumes {
				// If the requested volu
				if c, exists := usedVolumes[volumeId]; exists {
					log.Printf("The volume %s is used by the container %s. Impossible to remove it. Skipping.\n", volumeId, c.ID)
					continue
				}
				if err := srv.runtime.volumes.Delete(volumeId); err != nil {
					return err
				}
			}
		}
	} else {
		return fmt.Errorf("No such container: %s", name)
	}
	return nil
}

var ErrImageReferenced = errors.New("Image referenced by a repository")

func (srv *Server) deleteImageAndChildren(id string, imgs *[]APIRmi, byParents map[string][]*Image) error {
	// If the image is referenced by a repo, do not delete
	if len(srv.runtime.repositories.ByID()[id]) != 0 {
		return ErrImageReferenced
	}
	// If the image is not referenced but has children, go recursive
	referenced := false
	for _, img := range byParents[id] {
		if err := srv.deleteImageAndChildren(img.ID, imgs, byParents); err != nil {
			if err != ErrImageReferenced {
				return err
			}
			referenced = true
		}
	}
	if referenced {
		return ErrImageReferenced
	}

	// If the image is not referenced and has no children, remove it
	byParents, err := srv.runtime.graph.ByParent()
	if err != nil {
		return err
	}
	if len(byParents[id]) == 0 {
		if err := srv.runtime.repositories.DeleteAll(id); err != nil {
			return err
		}
		err := srv.runtime.graph.Delete(id)
		if err != nil {
			return err
		}
		*imgs = append(*imgs, APIRmi{Deleted: id})
		srv.LogEvent("delete", id, "")
		return nil
	}
	return nil
}

func (srv *Server) deleteImageParents(img *Image, imgs *[]APIRmi) error {
	if img.Parent != "" {
		parent, err := srv.runtime.graph.Get(img.Parent)
		if err != nil {
			return err
		}
		byParents, err := srv.runtime.graph.ByParent()
		if err != nil {
			return err
		}
		// Remove all children images
		if err := srv.deleteImageAndChildren(img.Parent, imgs, byParents); err != nil {
			return err
		}
		return srv.deleteImageParents(parent, imgs)
	}
	return nil
}

func (srv *Server) deleteImage(img *Image, repoName, tag string) ([]APIRmi, error) {
	imgs := []APIRmi{}
	tags := []string{}

	//If delete by id, see if the id belong only to one repository
	if repoName == "" {
		for _, repoAndTag := range srv.runtime.repositories.ByID()[img.ID] {
			parsedRepo, parsedTag := utils.ParseRepositoryTag(repoAndTag)
			if repoName == "" || repoName == parsedRepo {
				repoName = parsedRepo
				if parsedTag != "" {
					tags = append(tags, parsedTag)
				}
			} else if repoName != parsedRepo {
				// the id belongs to multiple repos, like base:latest and user:test,
				// in that case return conflict
				return imgs, nil
			}
		}
	} else {
		tags = append(tags, tag)
	}
	//Untag the current image
	for _, tag := range tags {
		tagDeleted, err := srv.runtime.repositories.Delete(repoName, tag)
		if err != nil {
			return nil, err
		}
		if tagDeleted {
			imgs = append(imgs, APIRmi{Untagged: img.ID})
			srv.LogEvent("untag", img.ID, "")
		}
	}
	if len(srv.runtime.repositories.ByID()[img.ID]) == 0 {
		if err := srv.deleteImageAndChildren(img.ID, &imgs, nil); err != nil {
			if err != ErrImageReferenced {
				return imgs, err
			}
		} else if err := srv.deleteImageParents(img, &imgs); err != nil {
			if err != ErrImageReferenced {
				return imgs, err
			}
		}
	}
	return imgs, nil
}

func (srv *Server) ImageDelete(name string, autoPrune bool) ([]APIRmi, error) {
	img, err := srv.runtime.repositories.LookupImage(name)
	if err != nil {
		return nil, fmt.Errorf("No such image: %s", name)
	}
	if !autoPrune {
		if err := srv.runtime.graph.Delete(img.ID); err != nil {
			return nil, fmt.Errorf("Cannot delete image %s: %s", name, err)
		}
		return nil, nil
	}

	// Prevent deletion if image is used by a running container
	for _, container := range srv.runtime.List() {
		if container.State.IsRunning() {
			parent, err := srv.runtime.repositories.LookupImage(container.Image)
			if err != nil {
				return nil, err
			}

			if err := parent.WalkHistory(func(p *Image) error {
				if img.ID == p.ID {
					return fmt.Errorf("Conflict, cannot delete %s because the running container %s is using it", name, container.ID)
				}
				return nil
			}); err != nil {
				return nil, err
			}
		}
	}

	if strings.Contains(img.ID, name) {
		//delete via ID
		return srv.deleteImage(img, "", "")
	}
	name, tag := utils.ParseRepositoryTag(name)
	return srv.deleteImage(img, name, tag)
}

func (srv *Server) ImageGetCached(imgID string, config *Config) (*Image, error) {

	// Retrieve all images
	images, err := srv.runtime.graph.Map()
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
	for elem := range imageMap[imgID] {
		img, err := srv.runtime.graph.Get(elem)
		if err != nil {
			return nil, err
		}
		if CompareConfig(&img.ContainerConfig, config) {
			return img, nil
		}
	}
	return nil, nil
}

func (srv *Server) RegisterLinks(name string, hostConfig *HostConfig) error {
	runtime := srv.runtime
	container := runtime.Get(name)
	if container == nil {
		return fmt.Errorf("No such container: %s", name)
	}

	if hostConfig != nil && hostConfig.Links != nil {
		for _, l := range hostConfig.Links {
			parts, err := parseLink(l)
			if err != nil {
				return err
			}
			child, err := srv.runtime.GetByName(parts["name"])
			if err != nil {
				return err
			}
			if child == nil {
				return fmt.Errorf("Could not get container for %s", parts["name"])
			}
			if err := runtime.RegisterLink(container, child, parts["alias"]); err != nil {
				return err
			}
		}

		// After we load all the links into the runtime
		// set them to nil on the hostconfig
		hostConfig.Links = nil
		if err := container.writeHostConfig(); err != nil {
			return err
		}
	}
	return nil
}

func (srv *Server) ContainerStart(job *engine.Job) engine.Status {
	if len(job.Args) < 1 {
		job.Errorf("Usage: %s container_id", job.Name)
		return engine.StatusErr
	}
	name := job.Args[0]
	runtime := srv.runtime
	container := runtime.Get(name)

	if container == nil {
		job.Errorf("No such container: %s", name)
		return engine.StatusErr
	}
	// If no environment was set, then no hostconfig was passed.
	if len(job.Environ()) > 0 {
		var hostConfig HostConfig
		if err := job.ExportEnv(&hostConfig); err != nil {
			job.Error(err)
			return engine.StatusErr
		}
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
				job.Errorf("Invalid bind mount '%s' : source can't be '/'", bind)
				return engine.StatusErr
			}

			// ensure the source exists on the host
			_, err := os.Stat(source)
			if err != nil && os.IsNotExist(err) {
				job.Errorf("Invalid bind mount '%s' : source doesn't exist", bind)
				return engine.StatusErr
			}
		}
		// Register any links from the host config before starting the container
		// FIXME: we could just pass the container here, no need to lookup by name again.
		if err := srv.RegisterLinks(name, &hostConfig); err != nil {
			job.Error(err)
			return engine.StatusErr
		}
		container.hostConfig = &hostConfig
		container.ToDisk()
	}
	if err := container.Start(); err != nil {
		job.Errorf("Cannot start container %s: %s", name, err)
		return engine.StatusErr
	}
	srv.LogEvent("start", container.ID, runtime.repositories.ImageName(container.Image))

	return engine.StatusOK
}

func (srv *Server) ContainerStop(name string, t int) error {
	if container := srv.runtime.Get(name); container != nil {
		if err := container.Stop(t); err != nil {
			return fmt.Errorf("Cannot stop container %s: %s", name, err)
		}
		srv.LogEvent("stop", container.ID, srv.runtime.repositories.ImageName(container.Image))
	} else {
		return fmt.Errorf("No such container: %s", name)
	}
	return nil
}

func (srv *Server) ContainerWait(name string) (int, error) {
	if container := srv.runtime.Get(name); container != nil {
		return container.Wait(), nil
	}
	return 0, fmt.Errorf("No such container: %s", name)
}

func (srv *Server) ContainerResize(name string, h, w int) error {
	if container := srv.runtime.Get(name); container != nil {
		return container.Resize(h, w)
	}
	return fmt.Errorf("No such container: %s", name)
}

func (srv *Server) ContainerAttach(name string, logs, stream, stdin, stdout, stderr bool, inStream io.ReadCloser, outStream, errStream io.Writer) error {
	container := srv.runtime.Get(name)
	if container == nil {
		return fmt.Errorf("No such container: %s", name)
	}

	//logs
	if logs {
		cLog, err := container.ReadLog("json")
		if err != nil && os.IsNotExist(err) {
			// Legacy logs
			utils.Errorf("Old logs format")
			if stdout {
				cLog, err := container.ReadLog("stdout")
				if err != nil {
					utils.Errorf("Error reading logs (stdout): %s", err)
				} else if _, err := io.Copy(outStream, cLog); err != nil {
					utils.Errorf("Error streaming logs (stdout): %s", err)
				}
			}
			if stderr {
				cLog, err := container.ReadLog("stderr")
				if err != nil {
					utils.Errorf("Error reading logs (stderr): %s", err)
				} else if _, err := io.Copy(errStream, cLog); err != nil {
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
					fmt.Fprintf(outStream, "%s", l.Log)
				}
				if l.Stream == "stderr" && stderr {
					fmt.Fprintf(errStream, "%s", l.Log)
				}
			}
		}
	}

	//stream
	if stream {
		if container.State.IsGhost() {
			return fmt.Errorf("Impossible to attach to a ghost container")
		}

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
				io.Copy(w, inStream)
			}()
			cStdin = r
			cStdinCloser = inStream
		}
		if stdout {
			cStdout = outStream
		}
		if stderr {
			cStderr = errStream
		}

		<-container.Attach(cStdin, cStdinCloser, cStdout, cStderr)

		// If we are in stdinonce mode, wait for the process to end
		// otherwise, simply return
		if container.Config.StdinOnce && !container.Config.Tty {
			container.Wait()
		}
	}
	return nil
}

func (srv *Server) ContainerInspect(name string) (*Container, error) {
	if container := srv.runtime.Get(name); container != nil {
		return container, nil
	}
	return nil, fmt.Errorf("No such container: %s", name)
}

func (srv *Server) ImageInspect(name string) (*Image, error) {
	if image, err := srv.runtime.repositories.LookupImage(name); err == nil && image != nil {
		return image, nil
	}
	return nil, fmt.Errorf("No such image: %s", name)
}

func (srv *Server) ContainerCopy(name string, resource string, out io.Writer) error {
	if container := srv.runtime.Get(name); container != nil {

		data, err := container.Copy(resource)
		if err != nil {
			return err
		}

		if _, err := io.Copy(out, data); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("No such container: %s", name)

}

func NewServer(eng *engine.Engine, config *DaemonConfig) (*Server, error) {
	runtime, err := NewRuntime(config)
	if err != nil {
		return nil, err
	}
	srv := &Server{
		Eng:         eng,
		runtime:     runtime,
		pullingPool: make(map[string]chan struct{}),
		pushingPool: make(map[string]chan struct{}),
		events:      make([]utils.JSONMessage, 0, 64), //only keeps the 64 last events
		listeners:   make(map[string]chan utils.JSONMessage),
		reqFactory:  nil,
	}
	runtime.srv = srv
	return srv, nil
}

func (srv *Server) HTTPRequestFactory(metaHeaders map[string][]string) *utils.HTTPRequestFactory {
	srv.Lock()
	defer srv.Unlock()
	if srv.reqFactory == nil {
		ud := utils.NewHTTPUserAgentDecorator(srv.versionInfos()...)
		md := &utils.HTTPMetaHeadersDecorator{
			Headers: metaHeaders,
		}
		factory := utils.NewHTTPRequestFactory(ud, md)
		srv.reqFactory = factory
	}
	return srv.reqFactory
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

type Server struct {
	sync.RWMutex
	runtime     *Runtime
	pullingPool map[string]chan struct{}
	pushingPool map[string]chan struct{}
	events      []utils.JSONMessage
	listeners   map[string]chan utils.JSONMessage
	reqFactory  *utils.HTTPRequestFactory
	Eng         *engine.Engine
}
