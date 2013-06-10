package docker

import (
	"fmt"
	"github.com/dotcloud/docker/auth"
	"github.com/dotcloud/docker/registry"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"
)

func (srv *Server) DockerVersion() APIVersion {
	return APIVersion{
		Version:   VERSION,
		GitCommit: GITCOMMIT,
		GoVersion: runtime.Version(),
	}
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

func (srv *Server) ImagesSearch(term string) ([]APISearch, error) {

	results, err := registry.NewRegistry(srv.runtime.root).SearchRepositories(term)
	if err != nil {
		return nil, err
	}

	var outs []APISearch
	for _, repo := range results.Results {
		var out APISearch
		out.Description = repo["description"]
		if len(out.Description) > 45 {
			out.Description = utils.Trunc(out.Description, 42) + "..."
		}
		out.Name = repo["name"]
		outs = append(outs, out)
	}
	return outs, nil
}

func (srv *Server) ImageInsert(name, url, path string, out io.Writer, sf *utils.StreamFormatter) (string, error) {
	out = utils.NewWriteFlusher(out)
	img, err := srv.runtime.repositories.LookupImage(name)
	if err != nil {
		return "", err
	}

	file, err := utils.Download(url, out)
	if err != nil {
		return "", err
	}
	defer file.Body.Close()

	config, _, err := ParseRun([]string{img.ID, "echo", "insert", url, path}, srv.runtime.capabilities)
	if err != nil {
		return "", err
	}

	b := NewBuilder(srv.runtime)
	c, err := b.Create(config)
	if err != nil {
		return "", err
	}

	if err := c.Inject(utils.ProgressReader(file.Body, int(file.ContentLength), out, sf.FormatProgress("Downloading", "%v/%v (%v)"), sf), path); err != nil {
		return "", err
	}
	// FIXME: Handle custom repo, tag comment, author
	img, err = b.Commit(c, "", "", img.Comment, img.Author, nil)
	if err != nil {
		return "", err
	}
	out.Write(sf.FormatStatus(img.ID))
	return img.ShortID(), nil
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
			out.Write([]byte(" \"" + parentImage.ShortID() + "\" -> \"" + image.ShortID() + "\"\n"))
		} else {
			out.Write([]byte(" base -> \"" + image.ShortID() + "\" [style=invis]\n"))
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
	outs := []APIImages{} //produce [] when empty instead of 'null'
	for name, repository := range srv.runtime.repositories.Repositories {
		if filter != "" && name != filter {
			continue
		}
		for tag, id := range repository {
			var out APIImages
			image, err := srv.runtime.graph.Get(id)
			if err != nil {
				log.Printf("Warning: couldn't load %s from %s/%s: %s", id, name, tag, err)
				continue
			}
			delete(allImages, id)
			out.Repository = name
			out.Tag = tag
			out.ID = image.ID
			out.Created = image.Created.Unix()
			outs = append(outs, out)
		}
	}
	// Display images which aren't part of a
	if filter == "" {
		for _, image := range allImages {
			var out APIImages
			out.ID = image.ID
			out.Created = image.Created.Unix()
			outs = append(outs, out)
		}
	}
	return outs, nil
}

func (srv *Server) DockerInfo() *APIInfo {
	images, _ := srv.runtime.graph.All()
	var imgcount int
	if images == nil {
		imgcount = 0
	} else {
		imgcount = len(images)
	}
	return &APIInfo{
		Containers:  len(srv.runtime.List()),
		Images:      imgcount,
		MemoryLimit: srv.runtime.capabilities.MemoryLimit,
		SwapLimit:   srv.runtime.capabilities.SwapLimit,
		Debug:       os.Getenv("DEBUG") != "",
		NFd:         utils.GetTotalUsedFds(),
		NGoroutines: runtime.NumGoroutine(),
	}
}

func (srv *Server) ImageHistory(name string) ([]APIHistory, error) {
	image, err := srv.runtime.repositories.LookupImage(name)
	if err != nil {
		return nil, err
	}

	outs := []APIHistory{} //produce [] when empty instead of 'null'
	err = image.WalkHistory(func(img *Image) error {
		var out APIHistory
		out.ID = srv.runtime.repositories.ImageName(img.ShortID())
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

func (srv *Server) Containers(all bool, n int, since, before string) []APIContainers {
	var foundBefore bool
	var displayed int
	retContainers := []APIContainers{}

	for _, container := range srv.runtime.List() {
		if !container.State.Running && !all && n == -1 && since == "" && before == "" {
			continue
		}
		if before != "" {
			if container.ShortID() == before {
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
		if container.ShortID() == since {
			break
		}
		displayed++

		c := APIContainers{
			ID: container.ID,
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
	return img.ShortID(), err
}

func (srv *Server) ContainerTag(name, repo, tag string, force bool) error {
	if err := srv.runtime.repositories.Set(repo, tag, name, force); err != nil {
		return err
	}
	return nil
}

func (srv *Server) pullImage(r *registry.Registry, out io.Writer, imgId, endpoint string, token []string, sf *utils.StreamFormatter) error {
	history, err := r.GetRemoteHistory(imgId, endpoint, token)
	if err != nil {
		return err
	}

	// FIXME: Try to stream the images?
	// FIXME: Launch the getRemoteImage() in goroutines
	for _, id := range history {
		if !srv.runtime.graph.Exists(id) {
			out.Write(sf.FormatStatus("Pulling %s metadata", id))
			imgJSON, err := r.GetRemoteImageJSON(id, endpoint, token)
			if err != nil {
				// FIXME: Keep goging in case of error?
				return err
			}
			img, err := NewImgJSON(imgJSON)
			if err != nil {
				return fmt.Errorf("Failed to parse json: %s", err)
			}

			// Get the layer
			out.Write(sf.FormatStatus("Pulling %s fs layer", id))
			layer, contentLength, err := r.GetRemoteImageLayer(img.ID, endpoint, token)
			if err != nil {
				return err
			}
			defer layer.Close()
			if err := srv.runtime.graph.Register(utils.ProgressReader(layer, contentLength, out, sf.FormatProgress("Downloading", "%v/%v (%v)"), sf), false, img); err != nil {
				return err
			}
		}
	}
	return nil
}

func (srv *Server) pullRepository(r *registry.Registry, out io.Writer, local, remote, askedTag string, sf *utils.StreamFormatter) error {
	out.Write(sf.FormatStatus("Pulling repository %s from %s", local, auth.IndexServerAddress()))
	repoData, err := r.GetRepositoryData(remote)
	if err != nil {
		return err
	}

	utils.Debugf("Updating checksums")
	// Reload the json file to make sure not to overwrite faster sums
	if err := srv.runtime.graph.UpdateChecksums(repoData.ImgList); err != nil {
		return err
	}

	utils.Debugf("Retrieving the tag list")
	tagsList, err := r.GetRemoteTags(repoData.Endpoints, remote, repoData.Tokens)
	if err != nil {
		return err
	}
	utils.Debugf("Registering tags")
	// If not specific tag have been asked, take all
	if askedTag == "" {
		for tag, id := range tagsList {
			repoData.ImgList[id].Tag = tag
		}
	} else {
		// Otherwise, check that the tag exists and use only that one
		id, exists := tagsList[askedTag]
		if !exists {
			return fmt.Errorf("Tag %s not found in repositoy %s", askedTag, local)
		}
		repoData.ImgList[id].Tag = askedTag
	}

	for _, img := range repoData.ImgList {
		if askedTag != "" && img.Tag != askedTag {
			utils.Debugf("(%s) does not match %s (id: %s), skipping", img.Tag, askedTag, img.ID)
			continue
		}
		out.Write(sf.FormatStatus("Pulling image %s (%s) from %s", img.ID, img.Tag, remote))
		success := false
		for _, ep := range repoData.Endpoints {
			if err := srv.pullImage(r, out, img.ID, "https://"+ep+"/v1", repoData.Tokens, sf); err != nil {
				out.Write(sf.FormatStatus("Error while retrieving image for tag: %s (%s); checking next endpoint", askedTag, err))
				continue
			}
			success = true
			break
		}
		if !success {
			return fmt.Errorf("Could not find repository on any of the indexed registries.")
		}
	}
	for tag, id := range tagsList {
		if askedTag != "" && tag != askedTag {
			continue
		}
		if err := srv.runtime.repositories.Set(local, tag, id, true); err != nil {
			return err
		}
	}
	if err := srv.runtime.repositories.Save(); err != nil {
		return err
	}

	return nil
}

func (srv *Server) ImagePull(name, tag, endpoint string, out io.Writer, sf *utils.StreamFormatter) error {
	r := registry.NewRegistry(srv.runtime.root)
	out = utils.NewWriteFlusher(out)
	if endpoint != "" {
		if err := srv.pullImage(r, out, name, endpoint, nil, sf); err != nil {
			return err
		}
		return nil
	}
	remote := name
	parts := strings.Split(name, "/")
	if len(parts) > 2 {
		remote = fmt.Sprintf("src/%s", url.QueryEscape(strings.Join(parts, "/")))
	}
	if err := srv.pullRepository(r, out, name, remote, tag, sf); err != nil {
		return err
	}

	return nil
}

// Retrieve the checksum of an image
// Priority:
// - Check on the stored checksums
// - Check if the archive exists, if it does not, ask the registry
// - If the archive does exists, process the checksum from it
// - If the archive does not exists and not found on registry, process checksum from layer
func (srv *Server) getChecksum(imageId string) (string, error) {
	// FIXME: Use in-memory map instead of reading the file each time
	if sums, err := srv.runtime.graph.getStoredChecksums(); err != nil {
		return "", err
	} else if checksum, exists := sums[imageId]; exists {
		return checksum, nil
	}

	img, err := srv.runtime.graph.Get(imageId)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(layerArchivePath(srv.runtime.graph.imageRoot(imageId))); err != nil {
		if os.IsNotExist(err) {
			// TODO: Ask the registry for the checksum
			//       As the archive is not there, it is supposed to come from a pull.
		} else {
			return "", err
		}
	}

	checksum, err := img.Checksum()
	if err != nil {
		return "", err
	}
	return checksum, nil
}

// Retrieve the all the images to be uploaded in the correct order
// Note: we can't use a map as it is not ordered
func (srv *Server) getImageList(localRepo map[string]string) ([]*registry.ImgData, error) {
	var imgList []*registry.ImgData

	imageSet := make(map[string]struct{})
	for tag, id := range localRepo {
		img, err := srv.runtime.graph.Get(id)
		if err != nil {
			return nil, err
		}
		img.WalkHistory(func(img *Image) error {
			if _, exists := imageSet[img.ID]; exists {
				return nil
			}
			imageSet[img.ID] = struct{}{}
			checksum, err := srv.getChecksum(img.ID)
			if err != nil {
				return err
			}
			imgList = append([]*registry.ImgData{{
				ID:       img.ID,
				Checksum: checksum,
				Tag:      tag,
			}}, imgList...)
			return nil
		})
	}
	return imgList, nil
}

func (srv *Server) pushRepository(r *registry.Registry, out io.Writer, name string, localRepo map[string]string, sf *utils.StreamFormatter) error {
	out = utils.NewWriteFlusher(out)
	out.Write(sf.FormatStatus("Processing checksums"))
	imgList, err := srv.getImageList(localRepo)
	if err != nil {
		return err
	}
	out.Write(sf.FormatStatus("Sending image list"))

	srvName := name
	parts := strings.Split(name, "/")
	if len(parts) > 2 {
		srvName = fmt.Sprintf("src/%s", url.QueryEscape(strings.Join(parts, "/")))
	}

	repoData, err := r.PushImageJSONIndex(srvName, imgList, false)
	if err != nil {
		return err
	}

	for _, ep := range repoData.Endpoints {
		out.Write(sf.FormatStatus("Pushing repository %s to %s (%d tags)", name, ep, len(localRepo)))
		// For each image within the repo, push them
		for _, elem := range imgList {
			if _, exists := repoData.ImgList[elem.ID]; exists {
				out.Write(sf.FormatStatus("Image %s already on registry, skipping", name))
				continue
			}
			if err := srv.pushImage(r, out, name, elem.ID, ep, repoData.Tokens, sf); err != nil {
				// FIXME: Continue on error?
				return err
			}
			out.Write(sf.FormatStatus("Pushing tags for rev [%s] on {%s}", elem.ID, ep+"/users/"+srvName+"/"+elem.Tag))
			if err := r.PushRegistryTag(srvName, elem.ID, elem.Tag, ep, repoData.Tokens); err != nil {
				return err
			}
		}
	}

	if _, err := r.PushImageJSONIndex(srvName, imgList, true); err != nil {
		return err
	}
	return nil
}

func (srv *Server) pushImage(r *registry.Registry, out io.Writer, remote, imgId, ep string, token []string, sf *utils.StreamFormatter) error {
	out = utils.NewWriteFlusher(out)
	jsonRaw, err := ioutil.ReadFile(path.Join(srv.runtime.graph.Root, imgId, "json"))
	if err != nil {
		return fmt.Errorf("Error while retreiving the path for {%s}: %s", imgId, err)
	}
	out.Write(sf.FormatStatus("Pushing %s", imgId))

	// Make sure we have the image's checksum
	checksum, err := srv.getChecksum(imgId)
	if err != nil {
		return err
	}
	imgData := &registry.ImgData{
		ID:       imgId,
		Checksum: checksum,
	}

	// Send the json
	if err := r.PushImageJSONRegistry(imgData, jsonRaw, ep, token); err != nil {
		if err == registry.ErrAlreadyExists {
			out.Write(sf.FormatStatus("Image %s already uploaded ; skipping", imgData.ID))
			return nil
		}
		return err
	}

	// Retrieve the tarball to be sent
	var layerData *TempArchive
	// If the archive exists, use it
	file, err := os.Open(layerArchivePath(srv.runtime.graph.imageRoot(imgId)))
	if err != nil {
		if os.IsNotExist(err) {
			// If the archive does not exist, create one from the layer
			layerData, err = srv.runtime.graph.TempLayerArchive(imgId, Xz, out)
			if err != nil {
				return fmt.Errorf("Failed to generate layer archive: %s", err)
			}
		} else {
			return err
		}
	} else {
		defer file.Close()
		st, err := file.Stat()
		if err != nil {
			return err
		}
		layerData = &TempArchive{
			File: file,
			Size: st.Size(),
		}
	}

	// Send the layer
	if err := r.PushImageLayerRegistry(imgData.ID, utils.ProgressReader(layerData, int(layerData.Size), out, sf.FormatProgress("Pushing", "%v/%v (%v)"), sf), ep, token); err != nil {
		return err
	}
	return nil
}

func (srv *Server) ImagePush(name, endpoint string, out io.Writer, sf *utils.StreamFormatter) error {
	out = utils.NewWriteFlusher(out)
	img, err := srv.runtime.graph.Get(name)
	r := registry.NewRegistry(srv.runtime.root)

	if err != nil {
		out.Write(sf.FormatStatus("The push refers to a repository [%s] (len: %d)", name, len(srv.runtime.repositories.Repositories[name])))
		// If it fails, try to get the repository
		if localRepo, exists := srv.runtime.repositories.Repositories[name]; exists {
			if err := srv.pushRepository(r, out, name, localRepo, sf); err != nil {
				return err
			}
			return nil
		}

		return err
	}
	out.Write(sf.FormatStatus("The push refers to an image: [%s]", name))
	if err := srv.pushImage(r, out, name, img.ID, endpoint, nil, sf); err != nil {
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
		out.Write(sf.FormatStatus("Downloading from %s", u))
		// Download with curl (pretty progress bar)
		// If curl is not available, fallback to http.Get()
		resp, err = utils.Download(u.String(), out)
		if err != nil {
			return err
		}
		archive = utils.ProgressReader(resp.Body, int(resp.ContentLength), out, sf.FormatProgress("Importing", "%v/%v (%v)"), sf)
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
	out.Write(sf.FormatStatus(img.ShortID()))
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
	return container.ShortID(), nil
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

func (srv *Server) ImageDelete(name string) error {
	img, err := srv.runtime.repositories.LookupImage(name)
	if err != nil {
		return fmt.Errorf("No such image: %s", name)
	}
	if err := srv.runtime.graph.Delete(img.ID); err != nil {
		return fmt.Errorf("Error deleting image %s: %s", name, err.Error())
	}
	return nil
}

func (srv *Server) ImageGetCached(imgId string, config *Config) (*Image, error) {

	// Retrieve all images
	images, err := srv.runtime.graph.All()
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
	for elem := range imageMap[imgId] {
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

func (srv *Server) ContainerResize(name string, h, w int) error {
	if container := srv.runtime.Get(name); container != nil {
		return container.Resize(h, w)
	}
	return fmt.Errorf("No such container: %s", name)
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
				utils.Debugf(err.Error())
			} else if _, err := io.Copy(out, cLog); err != nil {
				utils.Debugf(err.Error())
			}
		}
		if stderr {
			cLog, err := container.ReadLog("stderr")
			if err != nil {
				utils.Debugf(err.Error())
			} else if _, err := io.Copy(out, cLog); err != nil {
				utils.Debugf(err.Error())
			}
		}
	}

	//stream
	if stream {
		if container.State.Ghost {
			return fmt.Errorf("Impossible to attach to a ghost container")
		}
		if !container.State.Running {
			return fmt.Errorf("Impossible to attach to a stopped container, start it first")
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
	runtime.srv = srv
	return srv, nil
}

type Server struct {
	runtime *Runtime
}
