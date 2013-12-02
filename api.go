package docker

import (
	"bufio"
	"bytes"
	"code.google.com/p/go.net/websocket"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/auth"
	"github.com/dotcloud/docker/utils"
	"github.com/gorilla/mux"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const (
	APIVERSION        = 1.7
	DEFAULTHTTPHOST   = "127.0.0.1"
	DEFAULTHTTPPORT   = 4243
	DEFAULTUNIXSOCKET = "/var/run/docker.sock"
)

type HttpApiFunc func(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error

func hijackServer(w http.ResponseWriter) (io.ReadCloser, io.Writer, error) {
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return nil, nil, err
	}
	// Flush the options to make sure the client sets the raw mode
	conn.Write([]byte{})
	return conn, conn, nil
}

//If we don't do this, POST method without Content-type (even with empty body) will fail
func parseForm(r *http.Request) error {
	if r == nil {
		return nil
	}
	if err := r.ParseForm(); err != nil && !strings.HasPrefix(err.Error(), "mime:") {
		return err
	}
	return nil
}

func parseMultipartForm(r *http.Request) error {
	if err := r.ParseMultipartForm(4096); err != nil && !strings.HasPrefix(err.Error(), "mime:") {
		return err
	}
	return nil
}

func httpError(w http.ResponseWriter, err error) {
	statusCode := http.StatusInternalServerError
	// FIXME: this is brittle and should not be necessary.
	// If we need to differentiate between different possible error types, we should
	// create appropriate error types with clearly defined meaning.
	if strings.Contains(err.Error(), "No such") {
		statusCode = http.StatusNotFound
	} else if strings.HasPrefix(err.Error(), "Bad parameter") {
		statusCode = http.StatusBadRequest
	} else if strings.HasPrefix(err.Error(), "Conflict") {
		statusCode = http.StatusConflict
	} else if strings.HasPrefix(err.Error(), "Impossible") {
		statusCode = http.StatusNotAcceptable
	} else if strings.HasPrefix(err.Error(), "Wrong login/password") {
		statusCode = http.StatusUnauthorized
	} else if strings.Contains(err.Error(), "hasn't been activated") {
		statusCode = http.StatusForbidden
	}

	if err != nil {
		utils.Errorf("HTTP Error: statusCode=%d %s", statusCode, err.Error())
		http.Error(w, err.Error(), statusCode)
	}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) error {
	b, err := json.Marshal(v)

	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(b)

	return nil
}

func getBoolParam(value string) (bool, error) {
	if value == "" {
		return false, nil
	}
	ret, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("Bad parameter")
	}
	return ret, nil
}

func matchesContentType(contentType, expectedType string) bool {
	mimetype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		utils.Errorf("Error parsing media type: %s error: %s", contentType, err.Error())
	}
	return err == nil && mimetype == expectedType
}

func postAuth(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	authConfig := &auth.AuthConfig{}
	err := json.NewDecoder(r.Body).Decode(authConfig)
	if err != nil {
		return err
	}
	status, err := auth.Login(authConfig, srv.HTTPRequestFactory(nil))
	if err != nil {
		return err
	}
	if status != "" {
		return writeJSON(w, http.StatusOK, &APIAuth{Status: status})
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func getVersion(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return writeJSON(w, http.StatusOK, srv.DockerVersion())
}

func postContainersKill(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	if err := parseForm(r); err != nil {
		return err
	}
	name := vars["name"]

	signal := 0
	if r != nil {
		if s := r.Form.Get("signal"); s != "" {
			s, err := strconv.Atoi(s)
			if err != nil {
				return err
			}
			signal = s
		}
	}
	if err := srv.ContainerKill(name, signal); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func getContainersExport(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]

	if err := srv.ContainerExport(name, w); err != nil {
		utils.Errorf("%s", err)
		return err
	}
	return nil
}

func getImagesJSON(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	all, err := getBoolParam(r.Form.Get("all"))
	if err != nil {
		return err
	}
	filter := r.Form.Get("filter")

	outs, err := srv.Images(all, filter)
	if err != nil {
		return err
	}

	if version < 1.7 {
		outs2 := []APIImagesOld{}
		for _, ctnr := range outs {
			outs2 = append(outs2, ctnr.ToLegacy()...)
		}

		return writeJSON(w, http.StatusOK, outs2)
	}
	return writeJSON(w, http.StatusOK, outs)
}

func getImagesViz(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if version > 1.6 {
		w.WriteHeader(http.StatusNotFound)
		return fmt.Errorf("This is now implemented in the client.")
	}

	if err := srv.ImagesViz(w); err != nil {
		return err
	}
	return nil
}

func getInfo(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return writeJSON(w, http.StatusOK, srv.DockerInfo())
}

func getEvents(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	sendEvent := func(wf *utils.WriteFlusher, event *utils.JSONMessage) error {
		b, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("JSON error")
		}
		_, err = wf.Write(b)
		if err != nil {
			// On error, evict the listener
			utils.Errorf("%s", err)
			srv.Lock()
			delete(srv.listeners, r.RemoteAddr)
			srv.Unlock()
			return err
		}
		return nil
	}

	if err := parseForm(r); err != nil {
		return err
	}
	listener := make(chan utils.JSONMessage)
	srv.Lock()
	srv.listeners[r.RemoteAddr] = listener
	srv.Unlock()
	since, err := strconv.ParseInt(r.Form.Get("since"), 10, 0)
	if err != nil {
		since = 0
	}
	w.Header().Set("Content-Type", "application/json")
	wf := utils.NewWriteFlusher(w)
	wf.Flush()
	if since != 0 {
		// If since, send previous events that happened after the timestamp
		for _, event := range srv.GetEvents() {
			if event.Time >= since {
				err := sendEvent(wf, &event)
				if err != nil && err.Error() == "JSON error" {
					continue
				}
				if err != nil {
					return err
				}
			}
		}
	}
	for event := range listener {
		err := sendEvent(wf, &event)
		if err != nil && err.Error() == "JSON error" {
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func getImagesHistory(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]
	outs, err := srv.ImageHistory(name)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, outs)
}

func getContainersChanges(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]
	changesStr, err := srv.ContainerChanges(name)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, changesStr)
}

func getContainersTop(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if version < 1.4 {
		return fmt.Errorf("top was improved a lot since 1.3, Please upgrade your docker client.")
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	if err := parseForm(r); err != nil {
		return err
	}
	procsStr, err := srv.ContainerTop(vars["name"], r.Form.Get("ps_args"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, procsStr)
}

func getContainersJSON(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	all, err := getBoolParam(r.Form.Get("all"))
	if err != nil {
		return err
	}
	size, err := getBoolParam(r.Form.Get("size"))
	if err != nil {
		return err
	}
	since := r.Form.Get("since")
	before := r.Form.Get("before")
	n, err := strconv.Atoi(r.Form.Get("limit"))
	if err != nil {
		n = -1
	}

	outs := srv.Containers(all, size, n, since, before)

	if version < 1.5 {
		outs2 := []APIContainersOld{}
		for _, ctnr := range outs {
			outs2 = append(outs2, *ctnr.ToLegacy())
		}

		return writeJSON(w, http.StatusOK, outs2)
	}
	return writeJSON(w, http.StatusOK, outs)
}

func postImagesTag(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	repo := r.Form.Get("repo")
	tag := r.Form.Get("tag")
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]
	force, err := getBoolParam(r.Form.Get("force"))
	if err != nil {
		return err
	}

	if err := srv.ContainerTag(name, repo, tag, force); err != nil {
		return err
	}
	w.WriteHeader(http.StatusCreated)
	return nil
}

func postCommit(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	config := &Config{}
	if err := json.NewDecoder(r.Body).Decode(config); err != nil && err != io.EOF {
		utils.Errorf("%s", err)
	}
	repo := r.Form.Get("repo")
	tag := r.Form.Get("tag")
	container := r.Form.Get("container")
	author := r.Form.Get("author")
	comment := r.Form.Get("comment")
	id, err := srv.ContainerCommit(container, repo, tag, author, comment, config)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusCreated, &APIID{id})
}

// Creates an image from Pull or from Import
func postImagesCreate(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	src := r.Form.Get("fromSrc")
	image := r.Form.Get("fromImage")
	tag := r.Form.Get("tag")
	repo := r.Form.Get("repo")

	authEncoded := r.Header.Get("X-Registry-Auth")
	authConfig := &auth.AuthConfig{}
	if authEncoded != "" {
		authJson := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJson).Decode(authConfig); err != nil {
			// for a pull it is not an error if no auth was given
			// to increase compatibility with the existing api it is defaulting to be empty
			authConfig = &auth.AuthConfig{}
		}
	}
	if version > 1.0 {
		w.Header().Set("Content-Type", "application/json")
	}
	sf := utils.NewStreamFormatter(version > 1.0)
	if image != "" { //pull
		metaHeaders := map[string][]string{}
		for k, v := range r.Header {
			if strings.HasPrefix(k, "X-Meta-") {
				metaHeaders[k] = v
			}
		}
		if err := srv.ImagePull(image, tag, w, sf, authConfig, metaHeaders, version > 1.3); err != nil {
			if sf.Used() {
				w.Write(sf.FormatError(err))
				return nil
			}
			return err
		}
	} else { //import
		if err := srv.ImageImport(src, repo, tag, r.Body, w, sf); err != nil {
			if sf.Used() {
				w.Write(sf.FormatError(err))
				return nil
			}
			return err
		}
	}
	return nil
}

func getImagesSearch(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	term := r.Form.Get("term")
	outs, err := srv.ImagesSearch(term)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, outs)
}

func postImagesInsert(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	url := r.Form.Get("url")
	path := r.Form.Get("path")
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]
	if version > 1.0 {
		w.Header().Set("Content-Type", "application/json")
	}
	sf := utils.NewStreamFormatter(version > 1.0)
	err := srv.ImageInsert(name, url, path, w, sf)
	if err != nil {
		if sf.Used() {
			w.Write(sf.FormatError(err))
			return nil
		}
		return err
	}

	return nil
}

func postImagesPush(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	metaHeaders := map[string][]string{}
	for k, v := range r.Header {
		if strings.HasPrefix(k, "X-Meta-") {
			metaHeaders[k] = v
		}
	}
	if err := parseForm(r); err != nil {
		return err
	}
	authConfig := &auth.AuthConfig{}

	authEncoded := r.Header.Get("X-Registry-Auth")
	if authEncoded != "" {
		// the new format is to handle the authConfig as a header
		authJson := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJson).Decode(authConfig); err != nil {
			// to increase compatibility to existing api it is defaulting to be empty
			authConfig = &auth.AuthConfig{}
		}
	} else {
		// the old format is supported for compatibility if there was no authConfig header
		if err := json.NewDecoder(r.Body).Decode(authConfig); err != nil {
			return err
		}

	}

	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]
	if version > 1.0 {
		w.Header().Set("Content-Type", "application/json")
	}
	sf := utils.NewStreamFormatter(version > 1.0)
	if err := srv.ImagePush(name, w, sf, authConfig, metaHeaders); err != nil {
		if sf.Used() {
			w.Write(sf.FormatError(err))
			return nil
		}
		return err
	}
	return nil
}

func getImagesGet(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	name := vars["name"]
	if version > 1.0 {
		w.Header().Set("Content-Type", "application/x-tar")
	}
	return srv.ImageExport(name, w)
}

func postImagesLoad(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return srv.ImageLoad(r.Body)
}

func postContainersCreate(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return nil
	}
	out := &APIRun{}
	job := srv.Eng.Job("create", r.Form.Get("name"))
	if err := job.DecodeEnv(r.Body); err != nil {
		return err
	}
	resolvConf, err := utils.GetResolvConf()
	if err != nil {
		return err
	}
	if !job.GetenvBool("NetworkDisabled") && len(job.Getenv("Dns")) == 0 && len(srv.runtime.config.Dns) == 0 && utils.CheckLocalDns(resolvConf) {
		out.Warnings = append(out.Warnings, fmt.Sprintf("Docker detected local DNS server on resolv.conf. Using default external servers: %v", defaultDns))
		job.SetenvList("Dns", defaultDns)
	}
	// Read container ID from the first line of stdout
	job.Stdout.AddString(&out.ID)
	// Read warnings from stderr
	warnings := &bytes.Buffer{}
	job.Stderr.Add(warnings)
	if err := job.Run(); err != nil {
		return err
	}
	// Parse warnings from stderr
	scanner := bufio.NewScanner(warnings)
	for scanner.Scan() {
		out.Warnings = append(out.Warnings, scanner.Text())
	}
	if job.GetenvInt("Memory") > 0 && !srv.runtime.capabilities.MemoryLimit {
		log.Println("WARNING: Your kernel does not support memory limit capabilities. Limitation discarded.")
		out.Warnings = append(out.Warnings, "Your kernel does not support memory limit capabilities. Limitation discarded.")
	}
	if job.GetenvInt("Memory") > 0 && !srv.runtime.capabilities.SwapLimit {
		log.Println("WARNING: Your kernel does not support swap limit capabilities. Limitation discarded.")
		out.Warnings = append(out.Warnings, "Your kernel does not support memory swap capabilities. Limitation discarded.")
	}

	if !job.GetenvBool("NetworkDisabled") && srv.runtime.capabilities.IPv4ForwardingDisabled {
		log.Println("Warning: IPv4 forwarding is disabled.")
		out.Warnings = append(out.Warnings, "IPv4 forwarding is disabled.")
	}

	return writeJSON(w, http.StatusCreated, out)
}

func postContainersRestart(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	t, err := strconv.Atoi(r.Form.Get("t"))
	if err != nil || t < 0 {
		t = 10
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]
	if err := srv.ContainerRestart(name, t); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func deleteContainers(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]

	removeVolume, err := getBoolParam(r.Form.Get("v"))
	if err != nil {
		return err
	}
	removeLink, err := getBoolParam(r.Form.Get("link"))
	if err != nil {
		return err
	}

	if err := srv.ContainerDestroy(name, removeVolume, removeLink); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func deleteImages(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]
	imgs, err := srv.ImageDelete(name, version > 1.1)
	if err != nil {
		return err
	}
	if imgs != nil {
		if len(imgs) != 0 {
			return writeJSON(w, http.StatusOK, imgs)
		}
		return fmt.Errorf("Conflict, %s wasn't deleted", name)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func postContainersStart(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]
	job := srv.Eng.Job("start", name)
	// allow a nil body for backwards compatibility
	if r.Body != nil {
		if matchesContentType(r.Header.Get("Content-Type"), "application/json") {
			if err := job.DecodeEnv(r.Body); err != nil {
				return err
			}
		}
	}
	if err := job.Run(); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func postContainersStop(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	t, err := strconv.Atoi(r.Form.Get("t"))
	if err != nil || t < 0 {
		t = 10
	}

	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]

	if err := srv.ContainerStop(name, t); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func postContainersWait(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]

	status, err := srv.ContainerWait(name)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, &APIWait{StatusCode: status})
}

func postContainersResize(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	height, err := strconv.Atoi(r.Form.Get("h"))
	if err != nil {
		return err
	}
	width, err := strconv.Atoi(r.Form.Get("w"))
	if err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]
	if err := srv.ContainerResize(name, height, width); err != nil {
		return err
	}
	return nil
}

func postContainersAttach(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	logs, err := getBoolParam(r.Form.Get("logs"))
	if err != nil {
		return err
	}
	stream, err := getBoolParam(r.Form.Get("stream"))
	if err != nil {
		return err
	}
	stdin, err := getBoolParam(r.Form.Get("stdin"))
	if err != nil {
		return err
	}
	stdout, err := getBoolParam(r.Form.Get("stdout"))
	if err != nil {
		return err
	}
	stderr, err := getBoolParam(r.Form.Get("stderr"))
	if err != nil {
		return err
	}

	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]

	c, err := srv.ContainerInspect(name)
	if err != nil {
		return err
	}

	inStream, outStream, err := hijackServer(w)
	if err != nil {
		return err
	}
	defer func() {
		if tcpc, ok := inStream.(*net.TCPConn); ok {
			tcpc.CloseWrite()
		} else {
			inStream.Close()
		}
	}()
	defer func() {
		if tcpc, ok := outStream.(*net.TCPConn); ok {
			tcpc.CloseWrite()
		} else if closer, ok := outStream.(io.Closer); ok {
			closer.Close()
		}
	}()

	var errStream io.Writer

	fmt.Fprintf(outStream, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")

	if !c.Config.Tty && version >= 1.6 {
		errStream = utils.NewStdWriter(outStream, utils.Stderr)
		outStream = utils.NewStdWriter(outStream, utils.Stdout)
	} else {
		errStream = outStream
	}

	if err := srv.ContainerAttach(name, logs, stream, stdin, stdout, stderr, inStream, outStream, errStream); err != nil {
		fmt.Fprintf(outStream, "Error: %s\n", err)
	}
	return nil
}

func wsContainersAttach(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {

	if err := parseForm(r); err != nil {
		return err
	}
	logs, err := getBoolParam(r.Form.Get("logs"))
	if err != nil {
		return err
	}
	stream, err := getBoolParam(r.Form.Get("stream"))
	if err != nil {
		return err
	}
	stdin, err := getBoolParam(r.Form.Get("stdin"))
	if err != nil {
		return err
	}
	stdout, err := getBoolParam(r.Form.Get("stdout"))
	if err != nil {
		return err
	}
	stderr, err := getBoolParam(r.Form.Get("stderr"))
	if err != nil {
		return err
	}

	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]

	if _, err := srv.ContainerInspect(name); err != nil {
		return err
	}

	h := websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()

		if err := srv.ContainerAttach(name, logs, stream, stdin, stdout, stderr, ws, ws, ws); err != nil {
			utils.Errorf("Error: %s", err)
		}
	})
	h.ServeHTTP(w, r)

	return nil
}

func getContainersByName(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]

	container, err := srv.ContainerInspect(name)
	if err != nil {
		return err
	}

	_, err = srv.ImageInspect(name)
	if err == nil {
		return fmt.Errorf("Conflict between containers and images")
	}

	return writeJSON(w, http.StatusOK, container)
}

func getImagesByName(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]

	image, err := srv.ImageInspect(name)
	if err != nil {
		return err
	}

	_, err = srv.ContainerInspect(name)
	if err == nil {
		return fmt.Errorf("Conflict between containers and images")
	}

	return writeJSON(w, http.StatusOK, image)
}

func postBuild(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if version < 1.3 {
		return fmt.Errorf("Multipart upload for build is no longer supported. Please upgrade your docker client.")
	}
	remoteURL := r.FormValue("remote")
	repoName := r.FormValue("t")
	rawSuppressOutput := r.FormValue("q")
	rawNoCache := r.FormValue("nocache")
	rawRm := r.FormValue("rm")
	repoName, tag := utils.ParseRepositoryTag(repoName)

	var context io.Reader

	if remoteURL == "" {
		context = r.Body
	} else if utils.IsGIT(remoteURL) {
		if !strings.HasPrefix(remoteURL, "git://") {
			remoteURL = "https://" + remoteURL
		}
		root, err := ioutil.TempDir("", "docker-build-git")
		if err != nil {
			return err
		}
		defer os.RemoveAll(root)

		if output, err := exec.Command("git", "clone", remoteURL, root).CombinedOutput(); err != nil {
			return fmt.Errorf("Error trying to use git: %s (%s)", err, output)
		}

		c, err := archive.Tar(root, archive.Bzip2)
		if err != nil {
			return err
		}
		context = c
	} else if utils.IsURL(remoteURL) {
		f, err := utils.Download(remoteURL, ioutil.Discard)
		if err != nil {
			return err
		}
		defer f.Body.Close()
		dockerFile, err := ioutil.ReadAll(f.Body)
		if err != nil {
			return err
		}
		c, err := MkBuildContext(string(dockerFile), nil)
		if err != nil {
			return err
		}
		context = c
	}

	suppressOutput, err := getBoolParam(rawSuppressOutput)
	if err != nil {
		return err
	}
	noCache, err := getBoolParam(rawNoCache)
	if err != nil {
		return err
	}
	rm, err := getBoolParam(rawRm)
	if err != nil {
		return err
	}

	if version > 1.6 {
		w.Header().Set("Content-Type", "application/json")
	}
	sf := utils.NewStreamFormatter(version > 1.6)
	b := NewBuildFile(srv, utils.NewWriteFlusher(w), !suppressOutput, !noCache, rm, sf)
	id, err := b.Build(context)
	if err != nil {
		if sf.Used() {
			w.Write(sf.FormatError(err))
			return nil
		}
		return fmt.Errorf("Error build: %s", err)
	}
	if repoName != "" {
		srv.runtime.repositories.Set(repoName, tag, id, false)
	}
	return nil
}

func postContainersCopy(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]

	copyData := &APICopy{}
	contentType := r.Header.Get("Content-Type")
	if contentType == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(copyData); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("Content-Type not supported: %s", contentType)
	}

	if copyData.Resource == "" {
		return fmt.Errorf("Path cannot be empty")
	}
	if copyData.Resource[0] == '/' {
		copyData.Resource = copyData.Resource[1:]
	}

	if err := srv.ContainerCopy(name, copyData.Resource, w); err != nil {
		utils.Errorf("%s", err.Error())
		return err
	}
	return nil
}

func optionsHandler(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	w.WriteHeader(http.StatusOK)
	return nil
}
func writeCorsHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
	w.Header().Add("Access-Control-Allow-Methods", "GET, POST, DELETE, PUT, OPTIONS")
}

func makeHttpHandler(srv *Server, logging bool, localMethod string, localRoute string, handlerFunc HttpApiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// log the request
		utils.Debugf("Calling %s %s", localMethod, localRoute)

		if logging {
			log.Println(r.Method, r.RequestURI)
		}

		if strings.Contains(r.Header.Get("User-Agent"), "Docker-Client/") {
			userAgent := strings.Split(r.Header.Get("User-Agent"), "/")
			if len(userAgent) == 2 && userAgent[1] != VERSION {
				utils.Debugf("Warning: client and server don't have the same version (client: %s, server: %s)", userAgent[1], VERSION)
			}
		}
		version, err := strconv.ParseFloat(mux.Vars(r)["version"], 64)
		if err != nil {
			version = APIVERSION
		}
		if srv.runtime.config.EnableCors {
			writeCorsHeaders(w, r)
		}

		if version == 0 || version > APIVERSION {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if err := handlerFunc(srv, version, w, r, mux.Vars(r)); err != nil {
			utils.Errorf("Error: %s", err)
			httpError(w, err)
		}
	}
}

func AttachProfiler(router *mux.Router) {
	router.HandleFunc("/debug/pprof/", pprof.Index)
	router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	router.HandleFunc("/debug/pprof/heap", pprof.Handler("heap").ServeHTTP)
	router.HandleFunc("/debug/pprof/goroutine", pprof.Handler("goroutine").ServeHTTP)
	router.HandleFunc("/debug/pprof/threadcreate", pprof.Handler("threadcreate").ServeHTTP)
}

func createRouter(srv *Server, logging bool) (*mux.Router, error) {
	r := mux.NewRouter()
	if os.Getenv("DEBUG") != "" {
		AttachProfiler(r)
	}
	m := map[string]map[string]HttpApiFunc{
		"GET": {
			"/events":                         getEvents,
			"/info":                           getInfo,
			"/version":                        getVersion,
			"/images/json":                    getImagesJSON,
			"/images/viz":                     getImagesViz,
			"/images/search":                  getImagesSearch,
			"/images/{name:.*}/get":           getImagesGet,
			"/images/{name:.*}/history":       getImagesHistory,
			"/images/{name:.*}/json":          getImagesByName,
			"/containers/ps":                  getContainersJSON,
			"/containers/json":                getContainersJSON,
			"/containers/{name:.*}/export":    getContainersExport,
			"/containers/{name:.*}/changes":   getContainersChanges,
			"/containers/{name:.*}/json":      getContainersByName,
			"/containers/{name:.*}/top":       getContainersTop,
			"/containers/{name:.*}/attach/ws": wsContainersAttach,
		},
		"POST": {
			"/auth":                         postAuth,
			"/commit":                       postCommit,
			"/build":                        postBuild,
			"/images/create":                postImagesCreate,
			"/images/{name:.*}/insert":      postImagesInsert,
			"/images/load":                  postImagesLoad,
			"/images/{name:.*}/push":        postImagesPush,
			"/images/{name:.*}/tag":         postImagesTag,
			"/containers/create":            postContainersCreate,
			"/containers/{name:.*}/kill":    postContainersKill,
			"/containers/{name:.*}/restart": postContainersRestart,
			"/containers/{name:.*}/start":   postContainersStart,
			"/containers/{name:.*}/stop":    postContainersStop,
			"/containers/{name:.*}/wait":    postContainersWait,
			"/containers/{name:.*}/resize":  postContainersResize,
			"/containers/{name:.*}/attach":  postContainersAttach,
			"/containers/{name:.*}/copy":    postContainersCopy,
		},
		"DELETE": {
			"/containers/{name:.*}": deleteContainers,
			"/images/{name:.*}":     deleteImages,
		},
		"OPTIONS": {
			"": optionsHandler,
		},
	}

	for method, routes := range m {
		for route, fct := range routes {
			utils.Debugf("Registering %s, %s", method, route)
			// NOTE: scope issue, make sure the variables are local and won't be changed
			localRoute := route
			localFct := fct
			localMethod := method

			// build the handler function
			f := makeHttpHandler(srv, logging, localMethod, localRoute, localFct)

			// add the new route
			if localRoute == "" {
				r.Methods(localMethod).HandlerFunc(f)
			} else {
				r.Path("/v{version:[0-9.]+}" + localRoute).Methods(localMethod).HandlerFunc(f)
				r.Path(localRoute).Methods(localMethod).HandlerFunc(f)
			}
		}
	}

	return r, nil
}

// ServeRequest processes a single http request to the docker remote api.
// FIXME: refactor this to be part of Server and not require re-creating a new
// router each time. This requires first moving ListenAndServe into Server.
func ServeRequest(srv *Server, apiversion float64, w http.ResponseWriter, req *http.Request) error {
	router, err := createRouter(srv, false)
	if err != nil {
		return err
	}
	// Insert APIVERSION into the request as a convenience
	req.URL.Path = fmt.Sprintf("/v%g%s", apiversion, req.URL.Path)
	router.ServeHTTP(w, req)
	return nil
}

func ListenAndServe(proto, addr string, srv *Server, logging bool) error {
	log.Printf("Listening for HTTP on %s (%s)\n", addr, proto)

	r, err := createRouter(srv, logging)
	if err != nil {
		return err
	}
	l, e := net.Listen(proto, addr)
	if e != nil {
		return e
	}
	if proto == "unix" {
		if err := os.Chmod(addr, 0660); err != nil {
			return err
		}

		groups, err := ioutil.ReadFile("/etc/group")
		if err != nil {
			return err
		}
		re := regexp.MustCompile("(^|\n)docker:.*?:([0-9]+)")
		if gidMatch := re.FindStringSubmatch(string(groups)); gidMatch != nil {
			gid, err := strconv.Atoi(gidMatch[2])
			if err != nil {
				return err
			}
			utils.Debugf("docker group found. gid: %d", gid)
			if err := os.Chown(addr, 0, gid); err != nil {
				return err
			}
		}
	}
	httpSrv := http.Server{Addr: addr, Handler: r}
	return httpSrv.Serve(l)
}
