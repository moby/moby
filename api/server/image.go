package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/graph"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/ulimit"
	"github.com/docker/docker/pkg/version"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
	restful "github.com/emicklei/go-restful"
)

func (s *Server) postCommit(version version.Version, w *restful.Response, r *restful.Request) error {
	if err := parseForm(r.Request); err != nil {
		return err
	}

	if err := checkForJSON(r.Request); err != nil {
		return err
	}

	cname := r.Request.Form.Get("container")

	pause := boolValue(r.Request, "pause")
	if r.Request.FormValue("pause") == "" && version.GreaterThanOrEqualTo("1.13") {
		pause = true
	}

	c, _, err := runconfig.DecodeContainerConfig(r.Request.Body)
	if err != nil && err != io.EOF { //Do not fail if body is empty.
		return err
	}

	commitCfg := &builder.CommitConfig{
		Pause:   pause,
		Repo:    r.Request.Form.Get("repo"),
		Tag:     r.Request.Form.Get("tag"),
		Author:  r.Request.Form.Get("author"),
		Comment: r.Request.Form.Get("comment"),
		Changes: r.Request.Form["changes"],
		Config:  c,
	}

	imgID, err := builder.Commit(cname, s.daemon, commitCfg)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusCreated, &types.ContainerCommitResponse{
		ID: imgID,
	})
}

// Creates an image from Pull or from Import
func (s *Server) postImagesCreate(version version.Version, w *restful.Response, r *restful.Request) error {
	if err := parseForm(r.Request); err != nil {
		return err
	}

	var (
		image   = r.Request.Form.Get("fromImage")
		repo    = r.Request.Form.Get("repo")
		tag     = r.Request.Form.Get("tag")
		message = r.Request.Form.Get("message")
	)
	authEncoded := r.HeaderParameter("X-Registry-Auth")
	authConfig := &cliconfig.AuthConfig{}
	if authEncoded != "" {
		authJSON := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJSON).Decode(authConfig); err != nil {
			// for a pull it is not an error if no auth was given
			// to increase compatibility with the existing api it is defaulting to be empty
			authConfig = &cliconfig.AuthConfig{}
		}
	}

	var (
		err    error
		output = ioutils.NewWriteFlusher(w)
	)

	w.Header().Set("Content-Type", "application/json")

	if image != "" { //pull
		if tag == "" {
			image, tag = parsers.ParseRepositoryTag(image)
		}
		metaHeaders := map[string][]string{}
		for k, v := range r.Request.Header {
			if strings.HasPrefix(k, "X-Meta-") {
				metaHeaders[k] = v
			}
		}

		imagePullConfig := &graph.ImagePullConfig{
			MetaHeaders: metaHeaders,
			AuthConfig:  authConfig,
			OutStream:   output,
		}

		err = s.daemon.Repositories().Pull(image, tag, imagePullConfig)
	} else { //import
		if tag == "" {
			repo, tag = parsers.ParseRepositoryTag(repo)
		}

		src := r.Request.Form.Get("fromSrc")

		// 'err' MUST NOT be defined within this block, we need any error
		// generated from the download to be available to the output
		// stream processing below
		var newConfig *runconfig.Config
		newConfig, err = builder.BuildFromConfig(s.daemon, &runconfig.Config{}, r.Request.Form["changes"])
		if err != nil {
			return err
		}

		err = s.daemon.Repositories().Import(src, repo, tag, message, r.Request.Body, output, newConfig)
	}
	if err != nil {
		if !output.Flushed() {
			return err
		}
		sf := streamformatter.NewJSONStreamFormatter()
		output.Write(sf.FormatError(err))
	}

	return nil
}

func (s *Server) postImagesPush(version version.Version, w *restful.Response, r *restful.Request) error {
	metaHeaders := map[string][]string{}
	for k, v := range r.Request.Header {
		if strings.HasPrefix(k, "X-Meta-") {
			metaHeaders[k] = v
		}
	}
	if err := parseForm(r.Request); err != nil {
		return err
	}
	authConfig := &cliconfig.AuthConfig{}

	authEncoded := r.HeaderParameter("X-Registry-Auth")
	if authEncoded != "" {
		// the new format is to handle the authConfig as a header
		authJSON := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJSON).Decode(authConfig); err != nil {
			// to increase compatibility to existing api it is defaulting to be empty
			authConfig = &cliconfig.AuthConfig{}
		}
	} else {
		// the old format is supported for compatibility if there was no authConfig header
		if err := json.NewDecoder(r.Request.Body).Decode(authConfig); err != nil {
			return fmt.Errorf("Bad parameters and missing X-Registry-Auth: %v", err)
		}
	}

	name := r.PathParameter("name")
	output := ioutils.NewWriteFlusher(w)
	imagePushConfig := &graph.ImagePushConfig{
		MetaHeaders: metaHeaders,
		AuthConfig:  authConfig,
		Tag:         r.Request.Form.Get("tag"),
		OutStream:   output,
	}

	w.Header().Set("Content-Type", "application/json")

	if err := s.daemon.Repositories().Push(name, imagePushConfig); err != nil {
		if !output.Flushed() {
			return err
		}
		sf := streamformatter.NewJSONStreamFormatter()
		output.Write(sf.FormatError(err))
	}
	return nil
}

func (s *Server) getImagesGet(version version.Version, w *restful.Response, r *restful.Request) error {
	if err := parseForm(r.Request); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/x-tar")

	output := ioutils.NewWriteFlusher(w)
	var names []string
	if name := r.PathParameter("name"); name != "" {
		names = []string{name}
	} else {
		names = r.Request.Form["names"]
	}

	if err := s.daemon.Repositories().ImageExport(names, output); err != nil {
		if !output.Flushed() {
			return err
		}
		sf := streamformatter.NewJSONStreamFormatter()
		output.Write(sf.FormatError(err))
	}
	return nil
}

func (s *Server) postImagesLoad(version version.Version, w *restful.Response, r *restful.Request) error {
	return s.daemon.Repositories().Load(r.Request.Body, w)
}

func (s *Server) deleteImages(version version.Version, w *restful.Response, r *restful.Request) error {
	if err := parseForm(r.Request); err != nil {
		return err
	}

	name := r.PathParameter("name")

	if name == "" {
		return fmt.Errorf("image name cannot be blank")
	}

	force := boolValue(r.Request, "force")
	prune := !boolValue(r.Request, "noprune")

	list, err := s.daemon.ImageDelete(name, force, prune)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, list)
}

func (s *Server) getImagesByName(version version.Version, w *restful.Response, r *restful.Request) error {
	imageInspect, err := s.daemon.Repositories().Lookup(r.PathParameter("name"))
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, imageInspect)
}

func (s *Server) postBuild(version version.Version, w *restful.Response, r *restful.Request) error {
	var (
		authConfigs        = map[string]cliconfig.AuthConfig{}
		authConfigsEncoded = r.HeaderParameter("X-Registry-Config")
		buildConfig        = builder.NewBuildConfig()
	)

	if authConfigsEncoded != "" {
		authConfigsJSON := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authConfigsEncoded))
		if err := json.NewDecoder(authConfigsJSON).Decode(&authConfigs); err != nil {
			// for a pull it is not an error if no auth was given
			// to increase compatibility with the existing api it is defaulting
			// to be empty.
		}
	}

	w.Header().Set("Content-Type", "application/json")

	if boolValue(r.Request, "forcerm") && version.GreaterThanOrEqualTo("1.12") {
		buildConfig.Remove = true
	} else if r.Request.FormValue("rm") == "" && version.GreaterThanOrEqualTo("1.12") {
		buildConfig.Remove = true
	} else {
		buildConfig.Remove = boolValue(r.Request, "rm")
	}
	if boolValue(r.Request, "pull") && version.GreaterThanOrEqualTo("1.16") {
		buildConfig.Pull = true
	}

	output := ioutils.NewWriteFlusher(w)
	buildConfig.Stdout = output
	buildConfig.Context = r.Request.Body

	buildConfig.RemoteURL = r.Request.FormValue("remote")
	buildConfig.DockerfileName = r.Request.FormValue("dockerfile")
	buildConfig.RepoName = r.Request.FormValue("t")
	buildConfig.SuppressOutput = boolValue(r.Request, "q")
	buildConfig.NoCache = boolValue(r.Request, "nocache")
	buildConfig.ForceRemove = boolValue(r.Request, "forcerm")
	buildConfig.AuthConfigs = authConfigs
	buildConfig.MemorySwap = int64ValueOrZero(r.Request, "memswap")
	buildConfig.Memory = int64ValueOrZero(r.Request, "memory")
	buildConfig.CPUShares = int64ValueOrZero(r.Request, "cpushares")
	buildConfig.CPUPeriod = int64ValueOrZero(r.Request, "cpuperiod")
	buildConfig.CPUQuota = int64ValueOrZero(r.Request, "cpuquota")
	buildConfig.CPUSetCpus = r.Request.FormValue("cpusetcpus")
	buildConfig.CPUSetMems = r.Request.FormValue("cpusetmems")
	buildConfig.CgroupParent = r.Request.FormValue("cgroupparent")

	var buildUlimits = []*ulimit.Ulimit{}
	ulimitsJSON := r.Request.FormValue("ulimits")
	if ulimitsJSON != "" {
		if err := json.NewDecoder(strings.NewReader(ulimitsJSON)).Decode(&buildUlimits); err != nil {
			return err
		}
		buildConfig.Ulimits = buildUlimits
	}

	// Job cancellation. Note: not all job types support this.
	if closeNotifier, ok := w.ResponseWriter.(http.CloseNotifier); ok {
		finished := make(chan struct{})
		defer close(finished)
		go func() {
			select {
			case <-finished:
			case <-closeNotifier.CloseNotify():
				logrus.Infof("Client disconnected, cancelling job: build")
				buildConfig.Cancel()
			}
		}()
	}

	if err := builder.Build(s.daemon, buildConfig); err != nil {
		// Do not write the error in the http output if it's still empty.
		// This prevents from writing a 200(OK) when there is an interal error.
		if !output.Flushed() {
			return err
		}
		sf := streamformatter.NewJSONStreamFormatter()
		w.Write(sf.FormatError(err))
	}
	return nil
}

func (s *Server) getImagesJSON(version version.Version, w *restful.Response, r *restful.Request) error {
	if err := parseForm(r.Request); err != nil {
		return err
	}

	// FIXME: The filter parameter could just be a match filter
	images, err := s.daemon.Repositories().Images(r.Request.Form.Get("filters"), r.Request.Form.Get("filter"), boolValue(r.Request, "all"))
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, images)
}

func (s *Server) getImagesHistory(version version.Version, w *restful.Response, r *restful.Request) error {
	name := r.PathParameter("name")
	history, err := s.daemon.Repositories().History(name)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, history)
}

func (s *Server) postImagesTag(version version.Version, w *restful.Response, r *restful.Request) error {
	if err := parseForm(r.Request); err != nil {
		return err
	}

	repo := r.Request.Form.Get("repo")
	tag := r.Request.Form.Get("tag")
	force := boolValue(r.Request, "force")
	name := r.PathParameter("name")
	if err := s.daemon.Repositories().Tag(repo, tag, name, force); err != nil {
		return err
	}
	s.daemon.EventsService.Log("tag", utils.ImageReference(repo, tag), "")
	w.WriteHeader(http.StatusCreated)
	return nil
}

func (s *Server) getImagesSearch(version version.Version, w *restful.Response, r *restful.Request) error {
	if err := parseForm(r.Request); err != nil {
		return err
	}
	var (
		config      *cliconfig.AuthConfig
		authEncoded = r.HeaderParameter("X-Registry-Auth")
		headers     = map[string][]string{}
	)

	if authEncoded != "" {
		authJSON := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJSON).Decode(&config); err != nil {
			// for a search it is not an error if no auth was given
			// to increase compatibility with the existing api it is defaulting to be empty
			config = &cliconfig.AuthConfig{}
		}
	}
	for k, v := range r.Request.Header {
		if strings.HasPrefix(k, "X-Meta-") {
			headers[k] = v
		}
	}
	query, err := s.daemon.RegistryService.Search(r.Request.Form.Get("term"), config, headers)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, query.Results)
}
