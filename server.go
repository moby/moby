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

func (srv *Server) ContainerExport(name string, file *os.File) error {
	if container := srv.runtime.Get(name); container != nil {

		data, err := container.Export()
		if err != nil {
			return err
		}

		// Stream the entire contents of the container (basically a volatile snapshot)
		if _, err := io.Copy(file, data); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("No such container: %s", name)
}

func (srv *Server) Images(all, filter, quiet string) ([]ApiImages, error) {
	var allImages map[string]*Image
	var err error
	if all == "1" {
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
			if quiet != "1" {
				out.Repository = name
				out.Tag = tag
				out.Id = TruncateId(id)
				out.Created = image.Created.Unix()
			} else {
				out.Id = image.ShortId()
			}
			outs = append(outs, out)
		}
	}
	// Display images which aren't part of a
	if filter == "" {
		for id, image := range allImages {
			var out ApiImages
			if quiet != "1" {
				out.Repository = "<none>"
				out.Tag = "<none>"
				out.Id = TruncateId(id)
				out.Created = image.Created.Unix()
			} else {
				out.Id = image.ShortId()
			}
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
	if os.Getenv("DEBUG") == "1" {
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

func (srv *Server) ContainerChanges(name string) ([]string, error) {
	if container := srv.runtime.Get(name); container != nil {
		changes, err := container.Changes()
		if err != nil {
			return nil, err
		}
		var changesStr []string
		for _, name := range changes {
			changesStr = append(changesStr, name.String())
		}
		return changesStr, nil
	}
	return nil, fmt.Errorf("No such container: %s", name)
}

func (srv *Server) ContainerPort(name, privatePort string) (string, error) {
	if container := srv.runtime.Get(name); container != nil {
		if frontend, exists := container.NetworkSettings.PortMapping[privatePort]; exists {
			return frontend, nil
		}
		return "", fmt.Errorf("No private port '%s' allocated on %s", privatePort, name)
	}
	return "", fmt.Errorf("No such container: %s", name)
}

func (srv *Server) Containers(all, notrunc, quiet string, n int) []ApiContainers {
	var outs []ApiContainers = []ApiContainers{} //produce [] when empty instead of 'null'
	for i, container := range srv.runtime.List() {
		if !container.State.Running && all != "1" && n == -1 {
			continue
		}
		if i == n {
			break
		}
		var out ApiContainers
		out.Id = container.ShortId()
		if quiet != "1" {
			command := fmt.Sprintf("%s %s", container.Path, strings.Join(container.Args, " "))
			if notrunc != "1" {
				command = Trunc(command, 20)
			}
			out.Image = srv.runtime.repositories.ImageName(container.Image)
			out.Command = command
			out.Created = container.Created.Unix()
			out.Status = container.State.String()
			out.Ports = container.NetworkSettings.PortMappingHuman()
		}
		outs = append(outs, out)
	}
	return outs
}

func (srv *Server) ContainerCommit(name, repo, tag, author, comment string, config *Config) (string, error) {
	img, err := srv.runtime.Commit(name, repo, tag, comment, author, config)
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

func (srv *Server) ImagePull(name string, file *os.File) error {
	if srv.runtime.graph.LookupRemoteImage(name, srv.runtime.authConfig) {
		if err := srv.runtime.graph.PullImage(file, name, srv.runtime.authConfig); err != nil {
			return err
		}
	}
	if err := srv.runtime.graph.PullRepository(file, name, "", srv.runtime.repositories, srv.runtime.authConfig); err != nil {
		return err
	}
	return nil
}

func (srv *Server) ImagePush(name string, file *os.File) error {
	img, err := srv.runtime.graph.Get(name)
	if err != nil {
		Debugf("The push refers to a repository [%s] (len: %d)\n", name, len(srv.runtime.repositories.Repositories[name]))
		// If it fails, try to get the repository                                                                                                                      
		if localRepo, exists := srv.runtime.repositories.Repositories[name]; exists {
			if err := srv.runtime.graph.PushRepository(file, name, localRepo, srv.runtime.authConfig); err != nil {
				return err
			}
			return nil
		}

		return err
	}
	err = srv.runtime.graph.PushImage(file, img, srv.runtime.authConfig)
	if err != nil {
		return err
	}
	return nil
}

func (srv *Server) ImageImport(src, repo, tag string, file *os.File) error {
	var archive io.Reader
	var resp *http.Response

	if src == "-" {
		r, w := io.Pipe()
		go func() {
			defer w.Close()
			defer Debugf("Closing buffered stdin pipe")
			io.Copy(w, file)
		}()
		archive = r
	} else {
		u, err := url.Parse(src)
		if err != nil {
			fmt.Fprintln(file, "Error: "+err.Error())
		}
		if u.Scheme == "" {
			u.Scheme = "http"
			u.Host = src
			u.Path = ""
		}
		fmt.Fprintln(file, "Downloading from", u)
		// Download with curl (pretty progress bar)                                                                            
		// If curl is not available, fallback to http.Get()                                                                    
		resp, err = Download(u.String(), file)
		if err != nil {
			return err
		}
		archive = ProgressReader(resp.Body, int(resp.ContentLength), file, "Importing %v/%v (%v)")
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
	fmt.Fprintln(file, img.ShortId())
	return nil
}

func (srv *Server) ContainerCreate(config Config) (string, bool, bool, error) {
	var memoryW, swapW bool

	if config.Memory > 0 && !srv.runtime.capabilities.MemoryLimit {
		memoryW = true
		log.Println("WARNING: Your kernel does not support memory limit capabilities. Limitation discarded.")
		config.Memory = 0
	}

	if config.Memory > 0 && !srv.runtime.capabilities.SwapLimit {
		swapW = true
		log.Println("WARNING: Your kernel does not support swap limit capabilities. Limitation discarded.")
		config.MemorySwap = -1
	}
	container, err := srv.runtime.Create(&config)
	if err != nil {
		if srv.runtime.graph.IsNotExist(err) {
			return "", false, false, fmt.Errorf("No such image: %s", config.Image)
		}
		return "", false, false, err
	}
	return container.ShortId(), memoryW, swapW, nil
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

func (srv *Server) ContainerDestroy(name string, v bool) error {

	if container := srv.runtime.Get(name); container != nil {
		volumes := make(map[string]struct{})
		// Store all the deleted containers volumes
		for _, volumeId := range container.Volumes {
			volumes[volumeId] = struct{}{}
		}
		if err := srv.runtime.Destroy(container); err != nil {
			return fmt.Errorf("Error destroying container %s: %s", name, err.Error())
		}

		if v {
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
			return fmt.Errorf("Error deleteing image %s: %s", name, err.Error())
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

func (srv *Server) ContainerAttach(name, logs, stream, stdin, stdout, stderr string, file *os.File) error {
	if container := srv.runtime.Get(name); container != nil {
		//logs
		if logs == "1" {
			if stdout == "1" {
				cLog, err := container.ReadLog("stdout")
				if err != nil {
					Debugf(err.Error())
				} else if _, err := io.Copy(file, cLog); err != nil {
					Debugf(err.Error())
				}
			}
			if stderr == "1" {
				cLog, err := container.ReadLog("stderr")
				if err != nil {
					Debugf(err.Error())
				} else if _, err := io.Copy(file, cLog); err != nil {
					Debugf(err.Error())
				}
			}
		}

		//stream
		if stream == "1" {
			if container.State.Ghost {
				return fmt.Errorf("Impossible to attach to a ghost container")
			}

			if container.Config.Tty {
				oldState, err := SetRawTerminal()
				if err != nil {
					if os.Getenv("DEBUG") != "" {
						log.Printf("Can't set the terminal in raw mode: %s", err)
					}
				} else {
					defer RestoreTerminal(oldState)
				}
			}
			var (
				cStdin           io.ReadCloser
				cStdout, cStderr io.Writer
				cStdinCloser     io.Closer
			)

			if stdin == "1" {
				r, w := io.Pipe()
				go func() {
					defer w.Close()
					defer Debugf("Closing buffered stdin pipe")
					io.Copy(w, file)
				}()
				cStdin = r
				cStdinCloser = file
			}
			if stdout == "1" {
				cStdout = file
			}
			if stderr == "1" {
				cStderr = file
			}

			<-container.Attach(cStdin, cStdinCloser, cStdout, cStderr)
		}
	} else {
		return fmt.Errorf("No such container: %s", name)
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
