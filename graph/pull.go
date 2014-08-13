package graph

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/log"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

func (s *TagStore) CmdPull(job *engine.Job) engine.Status {
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

	c, err := s.poolAdd("pull", localName+":"+tag)
	if err != nil {
		if c != nil {
			// Another pull of the same repository is already taking place; just wait for it to finish
			job.Stdout.Write(sf.FormatStatus("", "Repository %s already being pulled by another client. Waiting.", localName))
			<-c
			return engine.StatusOK
		}
		return job.Error(err)
	}
	defer s.poolRemove("pull", localName+":"+tag)

	// Resolve the Repository name from fqn to endpoint + name
	hostname, remoteName, err := registry.ResolveRepositoryName(localName)
	if err != nil {
		return job.Error(err)
	}

	endpoint, err := registry.ExpandAndVerifyRegistryUrl(hostname)
	if err != nil {
		return job.Error(err)
	}

	r, err := registry.NewSession(authConfig, registry.HTTPRequestFactory(metaHeaders), endpoint, true)
	if err != nil {
		return job.Error(err)
	}

	if endpoint == registry.IndexServerAddress() {
		// If pull "index.docker.io/foo/bar", it's stored locally under "foo/bar"
		localName = remoteName
	}

	if err = s.pullRepository(r, job.Stdout, localName, remoteName, tag, sf, job.GetenvBool("parallel")); err != nil {
		return job.Error(err)
	}

	return engine.StatusOK
}

func (s *TagStore) pullRepository(r *registry.Session, out io.Writer, localName, remoteName, askedTag string, sf *utils.StreamFormatter, parallel bool) error {
	out.Write(sf.FormatStatus("", "Pulling repository %s", localName))

	repoData, err := r.GetRepositoryData(remoteName)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP code: 404") {
			return fmt.Errorf("Error: image %s not found", remoteName)
		} else {
			// Unexpected HTTP error
			return err
		}
	}

	log.Debugf("Retrieving the tag list")
	tagsList, err := r.GetRemoteTags(repoData.Endpoints, remoteName, repoData.Tokens)
	if err != nil {
		log.Errorf("%v", err)
		return err
	}

	for tag, id := range tagsList {
		repoData.ImgList[id] = &registry.ImgData{
			ID:       id,
			Tag:      tag,
			Checksum: "",
		}
	}

	log.Debugf("Registering tags")
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
				log.Debugf("(%s) does not match %s (id: %s), skipping", img.Tag, askedTag, img.ID)
				if parallel {
					errors <- nil
				}
				return
			}

			if img.Tag == "" {
				log.Debugf("Image (id: %s) present in this repository but untagged, skipping", img.ID)
				if parallel {
					errors <- nil
				}
				return
			}

			// ensure no two downloads of the same image happen at the same time
			if c, err := s.poolAdd("pull", "img:"+img.ID); err != nil {
				if c != nil {
					out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Layer already being pulled by another client. Waiting.", nil))
					<-c
					out.Write(sf.FormatProgress(utils.TruncateID(img.ID), "Download complete", nil))
				} else {
					log.Debugf("Image (id: %s) pull is already running, skipping: %v", img.ID, err)
				}
				if parallel {
					errors <- nil
				}
				return
			}
			defer s.poolRemove("pull", "img:"+img.ID)

			out.Write(sf.FormatProgress(utils.TruncateID(img.ID), fmt.Sprintf("Pulling image (%s) from %s", img.Tag, localName), nil))
			success := false
			var lastErr error
			for _, ep := range repoData.Endpoints {
				out.Write(sf.FormatProgress(utils.TruncateID(img.ID), fmt.Sprintf("Pulling image (%s) from %s, endpoint: %s", img.Tag, localName, ep), nil))
				if err := s.pullImage(r, out, img.ID, ep, repoData.Tokens, sf); err != nil {
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
				err := fmt.Errorf("Error pulling image (%s) from %s, %v", img.Tag, localName, lastErr)
				out.Write(sf.FormatProgress(utils.TruncateID(img.ID), err.Error(), nil))
				if parallel {
					errors <- err
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
		if err := s.Set(localName, tag, id, true); err != nil {
			return err
		}
	}

	return nil
}

func (s *TagStore) pullImage(r *registry.Session, out io.Writer, imgID, endpoint string, token []string, sf *utils.StreamFormatter) error {
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
		if c, err := s.poolAdd("pull", "layer:"+id); err != nil {
			log.Debugf("Image (id: %s) pull is already running, skipping: %v", id, err)
			<-c
		}
		defer s.poolRemove("pull", "layer:"+id)

		if !s.graph.Exists(id) {
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

				err = s.graph.Register(imgJSON,
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
