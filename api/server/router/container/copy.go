package container // import "github.com/moby/moby/api/server/router/container"

import (
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/moby/moby/api/server/httputils"
	"github.com/moby/moby/api/types"
	"github.com/moby/moby/api/types/versions"
	"github.com/moby/moby/errdefs"
	gddohttputil "github.com/golang/gddo/httputil"
)

type pathError struct{}

func (pathError) Error() string {
	return "Path cannot be empty"
}

func (pathError) InvalidParameter() {}

// postContainersCopy is deprecated in favor of getContainersArchive.
func (s *containerRouter) postContainersCopy(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	// Deprecated since 1.8, Errors out since 1.12
	version := httputils.VersionFromContext(ctx)
	if versions.GreaterThanOrEqualTo(version, "1.24") {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	if err := httputils.CheckForJSON(r); err != nil {
		return err
	}

	cfg := types.CopyConfig{}
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		if err == io.EOF {
			return errdefs.InvalidParameter(errors.New("got EOF while reading request body"))
		}
		return errdefs.InvalidParameter(err)
	}

	if cfg.Resource == "" {
		return pathError{}
	}

	data, err := s.backend.ContainerCopy(vars["name"], cfg.Resource)
	if err != nil {
		return err
	}
	defer data.Close()

	w.Header().Set("Content-Type", "application/x-tar")
	_, err = io.Copy(w, data)
	return err
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

func (s *containerRouter) headContainersArchive(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	v, err := httputils.ArchiveFormValues(r, vars)
	if err != nil {
		return err
	}

	stat, err := s.backend.ContainerStatPath(v.Name, v.Path)
	if err != nil {
		return err
	}

	return setContainerPathStatHeader(stat, w.Header())
}

func writeCompressedResponse(w http.ResponseWriter, r *http.Request, body io.Reader) error {
	var cw io.Writer
	switch gddohttputil.NegotiateContentEncoding(r, []string{"gzip", "deflate"}) {
	case "gzip":
		gw := gzip.NewWriter(w)
		defer gw.Close()
		cw = gw
		w.Header().Set("Content-Encoding", "gzip")
	case "deflate":
		fw, err := flate.NewWriter(w, flate.DefaultCompression)
		if err != nil {
			return err
		}
		defer fw.Close()
		cw = fw
		w.Header().Set("Content-Encoding", "deflate")
	default:
		cw = w
	}
	_, err := io.Copy(cw, body)
	return err
}

func (s *containerRouter) getContainersArchive(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	v, err := httputils.ArchiveFormValues(r, vars)
	if err != nil {
		return err
	}

	tarArchive, stat, err := s.backend.ContainerArchivePath(v.Name, v.Path)
	if err != nil {
		return err
	}
	defer tarArchive.Close()

	if err := setContainerPathStatHeader(stat, w.Header()); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/x-tar")
	return writeCompressedResponse(w, r, tarArchive)
}

func (s *containerRouter) putContainersArchive(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	v, err := httputils.ArchiveFormValues(r, vars)
	if err != nil {
		return err
	}

	noOverwriteDirNonDir := httputils.BoolValue(r, "noOverwriteDirNonDir")
	copyUIDGID := httputils.BoolValue(r, "copyUIDGID")

	return s.backend.ContainerExtractToDir(v.Name, v.Path, copyUIDGID, noOverwriteDirNonDir, r.Body)
}
