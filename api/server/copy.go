package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/version"
)

// postContainersCopy is deprecated in favor of getContainersArchive.
func (s *Server) postContainersCopy(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	if err := checkForJSON(r); err != nil {
		return err
	}

	cfg := types.CopyConfig{}
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		return err
	}

	if cfg.Resource == "" {
		return fmt.Errorf("Path cannot be empty")
	}

	data, err := s.daemon.ContainerCopy(vars["name"], cfg.Resource)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such id") {
			w.WriteHeader(http.StatusNotFound)
			return nil
		}
		if os.IsNotExist(err) {
			return fmt.Errorf("Could not find the file %s in container %s", cfg.Resource, vars["name"])
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

func (s *Server) headContainersArchive(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	v, err := archiveFormValues(r, vars)
	if err != nil {
		return err
	}

	stat, err := s.daemon.ContainerStatPath(v.name, v.path)
	if err != nil {
		return err
	}

	return setContainerPathStatHeader(stat, w.Header())
}

func (s *Server) getContainersArchive(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	v, err := archiveFormValues(r, vars)
	if err != nil {
		return err
	}

	tarArchive, stat, err := s.daemon.ContainerArchivePath(v.name, v.path)
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

func (s *Server) putContainersArchive(version version.Version, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	v, err := archiveFormValues(r, vars)
	if err != nil {
		return err
	}

	noOverwriteDirNonDir := boolValue(r, "noOverwriteDirNonDir")
	return s.daemon.ContainerExtractToDir(v.name, v.path, noOverwriteDirNonDir, r.Body)
}
