package docker

import (
	"code.google.com/p/go.net/websocket"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/auth"
	"github.com/dotcloud/docker/utils"
	"github.com/gorilla/mux"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const APIVERSION = 1.4
const DEFAULTHTTPHOST = "127.0.0.1"
const DEFAULTHTTPPORT = 4243
const DEFAULTUNIXSOCKET = "/var/run/docker.sock"

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
	if strings.HasPrefix(err.Error(), "No such") {
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
	utils.Debugf("[error %d] %s", statusCode, err)
	http.Error(w, err.Error(), statusCode)
}

func writeJSON(w http.ResponseWriter, b []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
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
		utils.Debugf("Error parsing media type: %s error: %s", contentType, err.Error())
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
		b, err := json.Marshal(&APIAuth{Status: status})
		if err != nil {
			return err
		}
		writeJSON(w, b)
		return nil
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func getVersion(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	m := srv.DockerVersion()
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	writeJSON(w, b)
	return nil
}

func postContainersKill(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]
	if err := srv.ContainerKill(name); err != nil {
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
		utils.Debugf("%s", err)
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
	b, err := json.Marshal(outs)
	if err != nil {
		return err
	}
	writeJSON(w, b)
	return nil
}

func getImagesViz(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := srv.ImagesViz(w); err != nil {
		return err
	}
	return nil
}

func getInfo(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	out := srv.DockerInfo()
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	writeJSON(w, b)
	return nil
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
			utils.Debugf("%s", err)
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
	if since != 0 {
		// If since, send previous events that happened after the timestamp
		for _, event := range srv.events {
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
	b, err := json.Marshal(outs)
	if err != nil {
		return err
	}
	writeJSON(w, b)
	return nil
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
	b, err := json.Marshal(changesStr)
	if err != nil {
		return err
	}
	writeJSON(w, b)
	return nil
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
	name := vars["name"]
	ps_args := r.Form.Get("ps_args")
	procsStr, err := srv.ContainerTop(name, ps_args)
	if err != nil {
		return err
	}
	b, err := json.Marshal(procsStr)
	if err != nil {
		return err
	}
	writeJSON(w, b)
	return nil
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
	b, err := json.Marshal(outs)
	if err != nil {
		return err
	}
	writeJSON(w, b)
	return nil
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
	if err := json.NewDecoder(r.Body).Decode(config); err != nil {
		utils.Debugf("%s", err)
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
	b, err := json.Marshal(&APIID{id})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, b)
	return nil
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

	authEncoded := r.Form.Get("authConfig")
	authConfig := &auth.AuthConfig{}
	if authEncoded != "" {
		authJson := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJson).Decode(authConfig); err != nil {
			// for a pull it is not an error if no auth was given
			// to increase compatibilit to existing api it is defaulting to be empty
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
	b, err := json.Marshal(outs)
	if err != nil {
		return err
	}
	writeJSON(w, b)
	return nil
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
	imgID, err := srv.ImageInsert(name, url, path, w, sf)
	if err != nil {
		if sf.Used() {
			w.Write(sf.FormatError(err))
			return nil
		}
	}
	b, err := json.Marshal(&APIID{ID: imgID})
	if err != nil {
		return err
	}
	writeJSON(w, b)
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

	authEncoded := r.Form.Get("authConfig")
	if authEncoded != "" {
		// the new format is to handle the authConfg as a parameter
		authJson := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJson).Decode(authConfig); err != nil {
			// for a pull it is not an error if no auth was given
			// to increase compatibilit to existing api it is defaulting to be empty
			authConfig = &auth.AuthConfig{}
		}
	} else {
		// the old format is supported for compatibility if there was no authConfig parameter
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

func postContainersCreate(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	config := &Config{}
	out := &APIRun{}

	if err := json.NewDecoder(r.Body).Decode(config); err != nil {
		return err
	}

	resolvConf, err := utils.GetResolvConf()
	if err != nil {
		return err
	}

	if len(config.Dns) == 0 && len(srv.runtime.Dns) == 0 && utils.CheckLocalDns(resolvConf) {
		out.Warnings = append(out.Warnings, fmt.Sprintf("Docker detected local DNS server on resolv.conf. Using default external servers: %v", defaultDns))
		config.Dns = defaultDns
	}

	id, err := srv.ContainerCreate(config)
	if err != nil {
		return err
	}
	out.ID = id

	if config.Memory > 0 && !srv.runtime.capabilities.MemoryLimit {
		log.Println("WARNING: Your kernel does not support memory limit capabilities. Limitation discarded.")
		out.Warnings = append(out.Warnings, "Your kernel does not support memory limit capabilities. Limitation discarded.")
	}
	if config.Memory > 0 && !srv.runtime.capabilities.SwapLimit {
		log.Println("WARNING: Your kernel does not support swap limit capabilities. Limitation discarded.")
		out.Warnings = append(out.Warnings, "Your kernel does not support memory swap capabilities. Limitation discarded.")
	}

	if srv.runtime.capabilities.IPv4ForwardingDisabled {
		log.Println("Warning: IPv4 forwarding is disabled.")
		out.Warnings = append(out.Warnings, "IPv4 forwarding is disabled.")
	}

	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, b)
	return nil
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

	if err := srv.ContainerDestroy(name, removeVolume); err != nil {
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
			b, err := json.Marshal(imgs)
			if err != nil {
				return err
			}
			writeJSON(w, b)
		} else {
			return fmt.Errorf("Conflict, %s wasn't deleted", name)
		}
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
	return nil
}

func postContainersStart(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	hostConfig := &HostConfig{}

	// allow a nil body for backwards compatibility
	if r.Body != nil {
		if matchesContentType(r.Header.Get("Content-Type"), "application/json") {
			if err := json.NewDecoder(r.Body).Decode(hostConfig); err != nil {
				return err
			}
		}
	}

	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]
	if err := srv.ContainerStart(name, hostConfig); err != nil {
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
	b, err := json.Marshal(&APIWait{StatusCode: status})
	if err != nil {
		return err
	}
	writeJSON(w, b)
	return nil
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

	if _, err := srv.ContainerInspect(name); err != nil {
		return err
	}

	in, out, err := hijackServer(w)
	if err != nil {
		return err
	}
	defer func() {
		if tcpc, ok := in.(*net.TCPConn); ok {
			tcpc.CloseWrite()
		} else {
			in.Close()
		}
	}()
	defer func() {
		if tcpc, ok := out.(*net.TCPConn); ok {
			tcpc.CloseWrite()
		} else if closer, ok := out.(io.Closer); ok {
			closer.Close()
		}
	}()

	fmt.Fprintf(out, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
	if err := srv.ContainerAttach(name, logs, stream, stdin, stdout, stderr, in, out); err != nil {
		fmt.Fprintf(out, "Error: %s\n", err)
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

		if err := srv.ContainerAttach(name, logs, stream, stdin, stdout, stderr, ws, ws); err != nil {
			utils.Debugf("Error: %s", err)
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
	b, err := json.Marshal(container)
	if err != nil {
		return err
	}
	writeJSON(w, b)
	return nil
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
	b, err := json.Marshal(image)
	if err != nil {
		return err
	}
	writeJSON(w, b)
	return nil
}

func postImagesGetCache(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	apiConfig := &APIImageConfig{}
	if err := json.NewDecoder(r.Body).Decode(apiConfig); err != nil {
		return err
	}

	image, err := srv.ImageGetCached(apiConfig.ID, apiConfig.Config)
	if err != nil {
		return err
	}
	if image == nil {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	apiID := &APIID{ID: image.ID}
	b, err := json.Marshal(apiID)
	if err != nil {
		return err
	}
	writeJSON(w, b)
	return nil
}

func postBuild(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if version < 1.3 {
		return fmt.Errorf("Multipart upload for build is no longer supported. Please upgrade your docker client.")
	}
	remoteURL := r.FormValue("remote")
	repoName := r.FormValue("t")
	rawSuppressOutput := r.FormValue("q")
	rawNoCache := r.FormValue("nocache")
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

		c, err := Tar(root, Bzip2)
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
		c, err := mkBuildContext(string(dockerFile), nil)
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

	b := NewBuildFile(srv, utils.NewWriteFlusher(w), !suppressOutput, !noCache)
	id, err := b.Build(context)
	if err != nil {
		fmt.Fprintf(w, "Error build: %s\n", err)
		return err
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
		return fmt.Errorf("Resource cannot be empty")
	}
	if copyData.Resource[0] == '/' {
		copyData.Resource = copyData.Resource[1:]
	}

	if err := srv.ContainerCopy(name, copyData.Resource, w); err != nil {
		utils.Debugf("%s", err.Error())
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
		if srv.enableCors {
			writeCorsHeaders(w, r)
		}

		if version == 0 || version > APIVERSION {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if err := handlerFunc(srv, version, w, r, mux.Vars(r)); err != nil {
			utils.Debugf("Error: %s", err)
			httpError(w, err)
		}
	}
}

func createRouter(srv *Server, logging bool) (*mux.Router, error) {
	r := mux.NewRouter()

	m := map[string]map[string]HttpApiFunc{
		"GET": {
			"/events":                         getEvents,
			"/info":                           getInfo,
			"/version":                        getVersion,
			"/images/json":                    getImagesJSON,
			"/images/viz":                     getImagesViz,
			"/images/search":                  getImagesSearch,
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
			"/images/{name:.*}/push":        postImagesPush,
			"/images/{name:.*}/tag":         postImagesTag,
			"/images/getCache":              postImagesGetCache,
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
