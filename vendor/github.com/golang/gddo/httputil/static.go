// Copyright 2013 The Go Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd.

package httputil

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"github.com/golang/gddo/httputil/header"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// StaticServer serves static files.
type StaticServer struct {
	// Dir specifies the location of the directory containing the files to serve.
	Dir string

	// MaxAge specifies the maximum age for the cache control and expiration
	// headers.
	MaxAge time.Duration

	// Error specifies the function used to generate error responses. If Error
	// is nil, then http.Error is used to generate error responses.
	Error Error

	// MIMETypes is a map from file extensions to MIME types.
	MIMETypes map[string]string

	mu    sync.Mutex
	etags map[string]string
}

func (ss *StaticServer) resolve(fname string) string {
	if path.IsAbs(fname) {
		panic("Absolute path not allowed when creating a StaticServer handler")
	}
	dir := ss.Dir
	if dir == "" {
		dir = "."
	}
	fname = filepath.FromSlash(fname)
	return filepath.Join(dir, fname)
}

func (ss *StaticServer) mimeType(fname string) string {
	ext := path.Ext(fname)
	var mimeType string
	if ss.MIMETypes != nil {
		mimeType = ss.MIMETypes[ext]
	}
	if mimeType == "" {
		mimeType = mime.TypeByExtension(ext)
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return mimeType
}

func (ss *StaticServer) openFile(fname string) (io.ReadCloser, int64, string, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, 0, "", err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, "", err
	}
	const modeType = os.ModeDir | os.ModeSymlink | os.ModeNamedPipe | os.ModeSocket | os.ModeDevice
	if fi.Mode()&modeType != 0 {
		f.Close()
		return nil, 0, "", errors.New("not a regular file")
	}
	return f, fi.Size(), ss.mimeType(fname), nil
}

// FileHandler returns a handler that serves a single file. The file is
// specified by a slash separated path relative to the static server's Dir
// field.
func (ss *StaticServer) FileHandler(fileName string) http.Handler {
	id := fileName
	fileName = ss.resolve(fileName)
	return &staticHandler{
		ss:   ss,
		id:   func(_ string) string { return id },
		open: func(_ string) (io.ReadCloser, int64, string, error) { return ss.openFile(fileName) },
	}
}

// DirectoryHandler returns a handler that serves files from a directory tree.
// The directory is specified by a slash separated path relative to the static
// server's Dir field.
func (ss *StaticServer) DirectoryHandler(prefix, dirName string) http.Handler {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	idBase := dirName
	dirName = ss.resolve(dirName)
	return &staticHandler{
		ss: ss,
		id: func(p string) string {
			if !strings.HasPrefix(p, prefix) {
				return "."
			}
			return path.Join(idBase, p[len(prefix):])
		},
		open: func(p string) (io.ReadCloser, int64, string, error) {
			if !strings.HasPrefix(p, prefix) {
				return nil, 0, "", errors.New("request url does not match directory prefix")
			}
			p = p[len(prefix):]
			return ss.openFile(filepath.Join(dirName, filepath.FromSlash(p)))
		},
	}
}

// FilesHandler returns a handler that serves the concatentation of the
// specified files. The files are specified by slash separated paths relative
// to the static server's Dir field.
func (ss *StaticServer) FilesHandler(fileNames ...string) http.Handler {

	// todo: cache concatenated files on disk and serve from there.

	mimeType := ss.mimeType(fileNames[0])
	var buf []byte
	var openErr error

	for _, fileName := range fileNames {
		p, err := ioutil.ReadFile(ss.resolve(fileName))
		if err != nil {
			openErr = err
			buf = nil
			break
		}
		buf = append(buf, p...)
	}

	id := strings.Join(fileNames, " ")

	return &staticHandler{
		ss: ss,
		id: func(_ string) string { return id },
		open: func(p string) (io.ReadCloser, int64, string, error) {
			return ioutil.NopCloser(bytes.NewReader(buf)), int64(len(buf)), mimeType, openErr
		},
	}
}

type staticHandler struct {
	id   func(fname string) string
	open func(p string) (io.ReadCloser, int64, string, error)
	ss   *StaticServer
}

func (h *staticHandler) error(w http.ResponseWriter, r *http.Request, status int, err error) {
	http.Error(w, http.StatusText(status), status)
}

func (h *staticHandler) etag(p string) (string, error) {
	id := h.id(p)

	h.ss.mu.Lock()
	if h.ss.etags == nil {
		h.ss.etags = make(map[string]string)
	}
	etag := h.ss.etags[id]
	h.ss.mu.Unlock()

	if etag != "" {
		return etag, nil
	}

	// todo: if a concurrent goroutine is calculating the hash, then wait for
	// it instead of computing it again here.

	rc, _, _, err := h.open(p)
	if err != nil {
		return "", err
	}

	defer rc.Close()

	w := sha1.New()
	_, err = io.Copy(w, rc)
	if err != nil {
		return "", err
	}

	etag = fmt.Sprintf(`"%x"`, w.Sum(nil))

	h.ss.mu.Lock()
	h.ss.etags[id] = etag
	h.ss.mu.Unlock()

	return etag, nil
}

func (h *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := path.Clean(r.URL.Path)
	if p != r.URL.Path {
		http.Redirect(w, r, p, 301)
		return
	}

	etag, err := h.etag(p)
	if err != nil {
		h.error(w, r, http.StatusNotFound, err)
		return
	}

	maxAge := h.ss.MaxAge
	if maxAge == 0 {
		maxAge = 24 * time.Hour
	}
	if r.FormValue("v") != "" {
		maxAge = 365 * 24 * time.Hour
	}

	cacheControl := fmt.Sprintf("public, max-age=%d", maxAge/time.Second)

	for _, e := range header.ParseList(r.Header, "If-None-Match") {
		if e == etag {
			w.Header().Set("Cache-Control", cacheControl)
			w.Header().Set("Etag", etag)
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	rc, cl, ct, err := h.open(p)
	if err != nil {
		h.error(w, r, http.StatusNotFound, err)
		return
	}
	defer rc.Close()

	w.Header().Set("Cache-Control", cacheControl)
	w.Header().Set("Etag", etag)
	if ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if cl != 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(cl, 10))
	}
	w.WriteHeader(http.StatusOK)
	if r.Method != "HEAD" {
		io.Copy(w, rc)
	}
}
