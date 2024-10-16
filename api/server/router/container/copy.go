package container // import "github.com/docker/docker/api/server/router/container"

import (
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types/container"
	gddohttputil "github.com/golang/gddo/httputil"
)

// setContainerPathStatHeader encodes the stat to JSON, base64 encode, and place in a header.
func setContainerPathStatHeader(stat *container.PathStat, header http.Header) error {
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

func (c *containerRouter) headContainersArchive(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	v, err := httputils.ArchiveFormValues(r, vars)
	if err != nil {
		return err
	}

	stat, err := c.backend.ContainerStatPath(v.Name, v.Path)
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

func (c *containerRouter) getContainersArchive(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	v, err := httputils.ArchiveFormValues(r, vars)
	if err != nil {
		return err
	}

	tarArchive, stat, err := c.backend.ContainerArchivePath(v.Name, v.Path)
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

func (c *containerRouter) putContainersArchive(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	v, err := httputils.ArchiveFormValues(r, vars)
	if err != nil {
		return err
	}

	noOverwriteDirNonDir := httputils.BoolValue(r, "noOverwriteDirNonDir")
	copyUIDGID := httputils.BoolValue(r, "copyUIDGID")

	return c.backend.ContainerExtractToDir(v.Name, v.Path, copyUIDGID, noOverwriteDirNonDir, r.Body)
}
