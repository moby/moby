package docker

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
)

func (srv *Server) DockerVersion() ApiVersion {
	return ApiVersion{VERSION, GIT_COMMIT, srv.runtime.capabilities.MemoryLimit, srv.runtime.capabilities.SwapLimit}
}

func (srv *Server) ContainerKill(name string) error {
	if container := srv.runtime.Get(name); container != nil {
		if err := container.Kill(); err != nil {
			return fmt.Errorf("Error restarting container %s: %s", name, err.Error())
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
		return nil
	}
	return fmt.Errorf("No such container: %s", name)
}

func (srv *Server) ImagesSearch(term string) ([]ApiSearch, error) {
	results, err := srv.runtime.graph.SearchRepositories(nil, term)
	if err != nil {
		return nil, err
	}

	var outs []ApiSearch
	for _, repo := range results.Results {
		var out ApiSearch
		out.Description = repo["description"]
		if len(out.Description) > 45 {
			out.Description = Trunc(out.Description, 42) + "..."
		}
		out.Name = repo["name"]
		outs = append(outs, out)
	}
	return outs, nil
}

func (srv *Server) ImageInsert(name, url, path string, out io.Writer) error {
	img, err := srv.runtime.repositories.LookupImage(name)
	if err != nil {
		return err
	}

	file, err := Download(url, out)
	if err != nil {
		return err
	}
	defer file.Body.Close()

	config, _, err := ParseRun([]string{img.Id, "echo", "insert", url, path}, srv.runtime.capabilities)
	if err != nil {
		return err
	}

	b := NewBuilder(srv.runtime)
	c, err := b.Create(config)
	if err != nil {
		return err
	}

	if err := c.Inject(ProgressReader(file.Body, int(file.ContentLength), out, "Downloading %v/%v (%v)"), path); err != nil {
		return err
	}
	// FIXME: Handle custom repo, tag comment, author
	img, err = b.Commit(c, "", "", img.Comment, img.Author, nil)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%s\n", img.Id)
	return nil
}

func (srv *Server) ImagesViz(out io.Writer) error {
	images, _ := srv.runtime.graph.All()
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
			out.Write([]byte(" \"" + parentImage.ShortId() + "\" -> \"" + image.ShortId() + "\"\n"))
		} else {
			out.Write([]byte(" base -> \"" + image.ShortId() + "\" [style=invis]\n"))
		}
	}

	reporefs := make(map[string][]string)

	for name, repository := range srv.runtime.repositories.Repositories {
		for tag, id := range repository {
			reporefs[TruncateId(id)] = append(reporefs[TruncateId(id)], fmt.Sprintf("%s:%s", name, tag))
		}
	}

	for id, repos := range reporefs {
		out.Write([]byte(" \"" + id + "\" [label=\"" + id + "\\n" + strings.Join(repos, "\\n") + "\",shape=box,fillcolor=\"paleturquoise\",style=\"filled,rounded\"];\n"))
	}
	out.Write([]byte(" base [style=invisible]\n}\n"))
	return nil
}

func (srv *Server) Images(all bool, filter string) ([]ApiImages, error) {
	var allImages map[string]*Image
	var err error
	if all {
		allImages, err = srv.runtime.graph.Map()
	} else {
		allImages, err = srv.runtime.graph.Heads()
	}
	if err != nil {
		return nil, err
	}
	var outs []ApiImages = []ApiImages{} //produce [] when empty instead of 'null'
	for name, repository := range srv.runtime.repositories.Repositories {
		if filter != "" && name != filter {
			continue
		}
		for tag, id := range repository {
			var out ApiImages
			image, err := srv.runtime.graph.Get(id)
			if err != nil {
				log.Printf("Warning: couldn't load %s from %s/%s: %s", id, name, tag, err)
				continue
			}
			delete(allImages, id)
			out.Repository = name
			out.Tag = tag
			out.Id = image.Id
			out.Created = image.Created.Unix()
			outs = append(outs, out)
		}
	}
	// Display images which aren't part of a
	if filter == "" {
		for _, image := range allImages {
			var out ApiImages
			out.Id = image.Id
			out.Created = image.Created.Unix()
			outs = append(outs, out)
		}
	}
	return outs, nil
}

func (srv *Server) DockerInfo() ApiInfo {
	images, _ := srv.runtime.graph.All()
	var imgcount int
	if images == nil {
		imgcount = 0
	} else {
		imgcount = len(images)
	}
	var out ApiInfo
	out.Containers = len(srv.runtime.List())
	out.Version = VERSION
	out.Images = imgcount
	out.GoVersion = runtime.Version()
	if os.Getenv("DEBUG") != "" {
		out.Debug = true
		out.NFd = getTotalUsedFds()
		out.NGoroutines = runtime.NumGoroutine()
	}
	return out
}

func (srv *Server) ImageHistory(name string) ([]ApiHistory, error) {
	image, err := srv.runtime.repositories.LookupImage(name)
	if err != nil {
		return nil, err
	}

	var outs []ApiHistory = []ApiHistory{} //produce [] when empty instead of 'null'
	err = image.WalkHistory(func(img *Image) error {
		var out ApiHistory
		out.Id = srv.runtime.repositories.ImageName(img.ShortId())
		out.Created = img.Created.Unix()
		out.CreatedBy = strings.Join(img.ContainerConfig.Cmd, " ")
		outs = append(outs, out)
		return nil
	})
	return outs, nil

}

func (srv *Server) ContainerChanges(name string) ([]Change, error) {
	if container := srv.runtime.Get(name); container != nil {
		return container.Changes()
	}
	return nil, fmt.Errorf("No such container: %s", name)
}

func (srv *Server) Containers(all bool, n int, since, before string) []ApiContainers {
	var foundBefore bool
	var displayed int
	retContainers := []ApiContainers{}

	for _, container := range srv.runtime.List() {
		if !container.State.Running && !all && n == -1 && since == "" && before == "" {
			continue
		}
		if before != "" {
			if container.ShortId() == before {
				foundBefore = true
				continue
			}
			if !foundBefore {
				continue
			}
		}
		if displayed == n {
			break
		}
		if container.ShortId() == since {
			break
		}
		displayed++

		c := ApiContainers{
			Id: container.Id,
		}
		c.Image = srv.runtime.repositories.ImageName(container.Image)
		c.Command = fmt.Sprintf("%s %s", container.Path, strings.Join(container.Args, " "))
		c.Created = container.Created.Unix()
		c.Status = container.State.String()
		c.Ports = container.NetworkSettings.PortMappingHuman()
		retContainers = append(retContainers, c)
	}
	return retContainers
}

func (srv *Server) ContainerCommit(name, repo, tag, author, comment string, config *Config) (string, error) {
	container := srv.runtime.Get(name)
	if container == nil {
		return "", fmt.Errorf("No such container: %s", name)
	}
	img, err := NewBuilder(srv.runtime).Commit(container, repo, tag, comment, author, config)
	if err != nil {
		return "", err
	}
	return img.ShortId(), err
}

func (srv *Server) ContainerTag(name, repo, tag string, force bool) error {
	if err := srv.runtime.repositories.Set(repo, tag, name, force); err != nil {
		return err
	}
	return nil
}

func (srv *Server) ImagePull(name, tag, registry string, out io.Writer) error {
	if registry != "" {
		if err := srv.runtime.graph.PullImage(out, name, registry, nil); err != nil {
			return err
		}
		return nil
	}
	if err := srv.runtime.graph.PullRepository(out, name, tag, srv.runtime.repositories, srv.runtime.authConfig); err != nil {
		return err
	}
	return nil
}

func (srv *Server) ImagePush(name, registry string, out io.Writer) error {
	img, err := srv.runtime.graph.Get(name)
	if err != nil {
		Debugf("The push refers to a repository [%s] (len: %d)\n", name, len(srv.runtime.repositories.Repositories[name]))
		// If it fails, try to get the repository
		if localRepo, exists := srv.runtime.repositories.Repositories[name]; exists {
			if err := srv.runtime.graph.PushRepository(out, name, localRepo, srv.runtime.authConfig); err != nil {
				return err
			}
			return nil
		}

		return err
	}
	err = srv.runtime.graph.PushImage(out, img, registry, nil)
	if err != nil {
		return err
	}
	return nil
}

func (srv *Server) ImageImport(src, repo, tag string, in io.Reader, out io.Writer) error {
	var archive io.Reader
	var resp *http.Response

	if src == "-" {
		archive = in
	} else {
		u, err := url.Parse(src)
		if err != nil {
			fmt.Fprintf(out, "Error: %s\n", err)
		}
		if u.Scheme == "" {
			u.Scheme = "http"
			u.Host = src
			u.Path = ""
		}
		fmt.Fprintln(out, "Downloading from", u)
		// Download with curl (pretty progress bar)
		// If curl is not available, fallback to http.Get()
		resp, err = Download(u.String(), out)
		if err != nil {
			return err
		}
		archive = ProgressReader(resp.Body, int(resp.ContentLength), out, "Importing %v/%v (%v)")
	}
	img, err := srv.runtime.graph.Create(archive, nil, "Imported from "+src, "", nil)
	if err != nil {
		return err
	}
	// Optionally register the image at REPO/TAG
	if repo != "" {
		if err := srv.runtime.repositories.Set(repo, tag, img.Id, true); err != nil {
			return err
		}
	}
	fmt.Fprintln(out, img.ShortId())
	return nil
}

func (srv *Server) ContainerCreate(config *Config) (string, error) {

	if config.Memory > 0 && !srv.runtime.capabilities.MemoryLimit {
		config.Memory = 0
	}

	if config.Memory > 0 && !srv.runtime.capabilities.SwapLimit {
		config.MemorySwap = -1
	}
	b := NewBuilder(srv.runtime)
	container, err := b.Create(config)
	if err != nil {
		if srv.runtime.graph.IsNotExist(err) {
			return "", fmt.Errorf("No such image: %s", config.Image)
		}
		return "", err
	}
	return container.ShortId(), nil
}

func (srv *Server) ImageCreateFromFile(dockerfile io.Reader, out io.Writer) error {
	img, err := NewBuilder(srv.runtime).Build(dockerfile, out)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%s\n", img.ShortId())
	return nil
}

func (srv *Server) ContainerRestart(name string, t int) error {
	if container := srv.runtime.Get(name); container != nil {
		if err := container.Restart(t); err != nil {
			return fmt.Errorf("Error restarting container %s: %s", name, err.Error())
		}
	} else {
		return fmt.Errorf("No such container: %s", name)
	}
	return nil
}

func (srv *Server) ContainerDestroy(name string, removeVolume bool) error {

	if container := srv.runtime.Get(name); container != nil {
		volumes := make(map[string]struct{})
		// Store all the deleted containers volumes
		for _, volumeId := range container.Volumes {
			volumes[volumeId] = struct{}{}
		}
		if err := srv.runtime.Destroy(container); err != nil {
			return fmt.Errorf("Error destroying container %s: %s", name, err.Error())
		}

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
					log.Printf("The volume %s is used by the container %s. Impossible to remove it. Skipping.\n", volumeId, c.Id)
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

func (srv *Server) ImageDelete(name string) error {
	img, err := srv.runtime.repositories.LookupImage(name)
	if err != nil {
		return fmt.Errorf("No such image: %s", name)
	} else {
		if err := srv.runtime.graph.Delete(img.Id); err != nil {
			return fmt.Errorf("Error deleting image %s: %s", name, err.Error())
		}
	}
	return nil
}

func (srv *Server) ContainerStart(name string) error {
	if container := srv.runtime.Get(name); container != nil {
		if err := container.Start(); err != nil {
			return fmt.Errorf("Error starting container %s: %s", name, err.Error())
		}
	} else {
		return fmt.Errorf("No such container: %s", name)
	}
	return nil
}

func (srv *Server) ContainerStop(name string, t int) error {
	if container := srv.runtime.Get(name); container != nil {
		if err := container.Stop(t); err != nil {
			return fmt.Errorf("Error stopping container %s: %s", name, err.Error())
		}
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

func (srv *Server) ContainerAttach(name string, logs, stream, stdin, stdout, stderr bool, in io.ReadCloser, out io.Writer) error {
	container := srv.runtime.Get(name)
	if container == nil {
		return fmt.Errorf("No such container: %s", name)
	}

	//logs
	if logs {
		if stdout {
			cLog, err := container.ReadLog("stdout")
			if err != nil {
				Debugf(err.Error())
			} else if _, err := io.Copy(out, cLog); err != nil {
				Debugf(err.Error())
			}
		}
		if stderr {
			cLog, err := container.ReadLog("stderr")
			if err != nil {
				Debugf(err.Error())
			} else if _, err := io.Copy(out, cLog); err != nil {
				Debugf(err.Error())
			}
		}
	}

	//stream
	if stream {
		if container.State.Ghost {
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
				defer Debugf("Closing buffered stdin pipe")
				io.Copy(w, in)
			}()
			cStdin = r
			cStdinCloser = in
		}
		if stdout {
			cStdout = out
		}
		if stderr {
			cStderr = out
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

func NewServer(autoRestart bool) (*Server, error) {
	if runtime.GOARCH != "amd64" {
		log.Fatalf("The docker runtime currently only supports amd64 (not %s). This will change in the future. Aborting.", runtime.GOARCH)
	}
	runtime, err := NewRuntime(autoRestart)
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
