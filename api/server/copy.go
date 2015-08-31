package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/docker/docker/api/middleware"
	"github.com/docker/docker/api/types"
)

// postContainersCopy is deprecated in favor of getContainersArchive.
func (s *Server) postContainersCopy(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	if err := checkForJSON(r.Request); err != nil {
		return err
	}

	cfg := types.CopyConfig{}
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		return err
	}

	res := cfg.Resource
	if res == "" {
		return fmt.Errorf("Path cannot be empty")
	}

	if res[0] == '/' || res[0] == '\\' {
		res = res[1:]
	}

	data, err := r.Container.Copy(res)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such id") {
			w.WriteHeader(http.StatusNotFound)
			return nil
		}
		if os.IsNotExist(err) {
			return fmt.Errorf("Could not find the file %s in container %s", cfg.Resource, r.Container.ID)
		}
		return err
	}
	defer data.Close()

	w.Header().Set("Content-Type", "application/x-tar")
	if _, err := io.Copy(w, data); err != nil {
		return err
	}

	return nil
}

// // Encode the stat to JSON, base64 encode, and place in a header.
func setContainerPathStatHeader(stat *types.ContainerPathStat, header http.Header) error {
	statJSON, err := json.Marshal(stat)
	if err != nil {
		return err
	}

	header.Set(
		"X-Docker-Container-Path-Stat",
		base64.StdEncoding.EncodeToString(statJSON),
	)

	return nil
}

// headContainersArchive stats the filesystem resource at the specified path in the
// container identified by the given name.
func (s *Server) headContainersArchive(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	path, err := parsePathParameter(r)
	if err != nil {
		return err
	}

	stat, err := r.Container.StatPath(path)
	if err != nil {
		return err
	}

	return setContainerPathStatHeader(stat, w.Header())
}

func (s *Server) getContainersArchive(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	path, err := parsePathParameter(r)
	if err != nil {
		return err
	}

	tarArchive, stat, err := r.Container.ArchivePath(path)
	if err != nil {
		return err
	}
	defer tarArchive.Close()

	if err := setContainerPathStatHeader(stat, w.Header()); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/x-tar")
	_, err = io.Copy(w, tarArchive)

	return err
}

// putContainersArchive extracts the given archive to the specified location
// in the filesystem of the container identified by the given name. The given
// path must be of a directory in the container. If it is not, the error will
// be ErrExtractPointNotDirectory. If noOverwriteDirNonDir is true then it will
// be an error if unpacking the given content would cause an existing directory
// to be replaced with a non-directory and vice versa.
func (s *Server) putContainersArchive(w http.ResponseWriter, r *middleware.ContainerRequest) error {
	path, err := parsePathParameter(r)
	if err != nil {
		return err
	}

	noOverwriteDirNonDir := boolValue(r, "noOverwriteDirNonDir")
	return r.Container.ExtractToDir(path, noOverwriteDirNonDir, r.Body)
}
