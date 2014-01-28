package docker

import (
	"bufio"
	"bytes"
	"code.google.com/p/go.net/websocket"
	"encoding/base64"
	"encoding/json"
	"expvar"
	"fmt"
	"github.com/dotcloud/docker/auth"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/pkg/systemd"
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
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

const (
	APIVERSION        = 1.9
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
	} else if strings.Contains(err.Error(), "Bad parameter") {
		statusCode = http.StatusBadRequest
	} else if strings.Contains(err.Error(), "Conflict") {
		statusCode = http.StatusConflict
	} else if strings.Contains(err.Error(), "Impossible") {
		statusCode = http.StatusNotAcceptable
	} else if strings.Contains(err.Error(), "Wrong login/password") {
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
	var (
		authConfig, err = ioutil.ReadAll(r.Body)
		job             = srv.Eng.Job("auth")
		status          string
	)
	if err != nil {
		return err
	}
	job.Setenv("authConfig", string(authConfig))
	job.Stdout.AddString(&status)
	if err = job.Run(); err != nil {
		return err
	}
	if status != "" {
		var env engine.Env
		env.Set("Status", status)
		return writeJSON(w, http.StatusOK, env)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func getVersion(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	w.Header().Set("Content-Type", "application/json")
	srv.Eng.ServeHTTP(w, r)
	return nil
}

func postContainersKill(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	if err := parseForm(r); err != nil {
		return err
	}
	job := srv.Eng.Job("kill", vars["name"])
	if sig := r.Form.Get("signal"); sig != "" {
		job.Args = append(job.Args, sig)
	}
	if err := job.Run(); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func getContainersExport(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	job := srv.Eng.Job("export", vars["name"])
	job.Stdout.Add(w)
	if err := job.Run(); err != nil {
		return err
	}
	return nil
}

func getImagesJSON(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	var (
		err  error
		outs *engine.Table
		job  = srv.Eng.Job("images")
	)

	job.Setenv("filter", r.Form.Get("filter"))
	job.Setenv("all", r.Form.Get("all"))

	if version > 1.8 {
		job.Stdout.Add(w)
	} else if outs, err = job.Stdout.AddListTable(); err != nil {
		return err
	}

	if err := job.Run(); err != nil {
		return err
	}

	if version < 1.8 && outs != nil { // Convert to legacy format
		outsLegacy := engine.NewTable("Created", 0)
		for _, out := range outs.Data {
			for _, repoTag := range out.GetList("RepoTags") {
				parts := strings.Split(repoTag, ":")
				outLegacy := &engine.Env{}
				outLegacy.Set("Repository", parts[0])
				outLegacy.Set("Tag", parts[1])
				outLegacy.Set("ID", out.Get("ID"))
				outLegacy.SetInt64("Created", out.GetInt64("Created"))
				outLegacy.SetInt64("Size", out.GetInt64("Size"))
				outLegacy.SetInt64("VirtualSize", out.GetInt64("VirtualSize"))
				outsLegacy.Add(outLegacy)
			}
		}
		if _, err := outsLegacy.WriteListTo(w); err != nil {
			return err
		}
	}
	return nil
}

func getImagesViz(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if version > 1.6 {
		w.WriteHeader(http.StatusNotFound)
		return fmt.Errorf("This is now implemented in the client.")
	}
	srv.Eng.ServeHTTP(w, r)
	return nil
}

func getInfo(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	w.Header().Set("Content-Type", "application/json")
	srv.Eng.ServeHTTP(w, r)
	return nil
}

func getEvents(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	var job = srv.Eng.Job("events", r.RemoteAddr)
	job.Stdout.Add(utils.NewWriteFlusher(w))
	job.Setenv("since", r.Form.Get("since"))
	return job.Run()
}

func getImagesHistory(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	var job = srv.Eng.Job("history", vars["name"])
	job.Stdout.Add(w)

	if err := job.Run(); err != nil {
		return err
	}
	return nil
}

func getContainersChanges(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	var job = srv.Eng.Job("changes", vars["name"])
	job.Stdout.Add(w)

	return job.Run()
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

	job := srv.Eng.Job("top", vars["name"], r.Form.Get("ps_args"))
	job.Stdout.Add(w)
	return job.Run()
}

func getContainersJSON(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	var (
		err  error
		outs *engine.Table
		job  = srv.Eng.Job("containers")
	)

	job.Setenv("all", r.Form.Get("all"))
	job.Setenv("size", r.Form.Get("size"))
	job.Setenv("since", r.Form.Get("since"))
	job.Setenv("before", r.Form.Get("before"))
	job.Setenv("limit", r.Form.Get("limit"))

	if version > 1.5 {
		job.Stdout.Add(w)
	} else if outs, err = job.Stdout.AddTable(); err != nil {
		return err
	}
	if err = job.Run(); err != nil {
		return err
	}
	if version < 1.5 { // Convert to legacy format
		for _, out := range outs.Data {
			ports := engine.NewTable("", 0)
			ports.ReadListFrom([]byte(out.Get("Ports")))
			out.Set("Ports", displayablePorts(ports))
		}
		if _, err = outs.WriteListTo(w); err != nil {
			return err
		}
	}
	return nil
}

func postImagesTag(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	job := srv.Eng.Job("tag", vars["name"], r.Form.Get("repo"), r.Form.Get("tag"))
	job.Setenv("force", r.Form.Get("force"))
	if err := job.Run(); err != nil {
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

	job := srv.Eng.Job("commit", r.Form.Get("container"))
	job.Setenv("repo", r.Form.Get("repo"))
	job.Setenv("tag", r.Form.Get("tag"))
	job.Setenv("author", r.Form.Get("author"))
	job.Setenv("comment", r.Form.Get("comment"))
	job.SetenvJson("config", config)

	var id string
	job.Stdout.AddString(&id)
	if err := job.Run(); err != nil {
		return err
	}

	return writeJSON(w, http.StatusCreated, &APIID{id})
}

// Creates an image from Pull or from Import
func postImagesCreate(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	var (
		image = r.Form.Get("fromImage")
		tag   = r.Form.Get("tag")
		job   *engine.Job
	)
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
	if image != "" { //pull
		metaHeaders := map[string][]string{}
		for k, v := range r.Header {
			if strings.HasPrefix(k, "X-Meta-") {
				metaHeaders[k] = v
			}
		}
		job = srv.Eng.Job("pull", r.Form.Get("fromImage"), tag)
		job.SetenvBool("parallel", version > 1.3)
		job.SetenvJson("metaHeaders", metaHeaders)
		job.SetenvJson("authConfig", authConfig)
	} else { //import
		job = srv.Eng.Job("import", r.Form.Get("fromSrc"), r.Form.Get("repo"), tag)
		job.Stdin.Add(r.Body)
	}

	job.SetenvBool("json", version > 1.0)
	job.Stdout.Add(utils.NewWriteFlusher(w))
	if err := job.Run(); err != nil {
		if !job.Stdout.Used() {
			return err
		}
		sf := utils.NewStreamFormatter(version > 1.0)
		w.Write(sf.FormatError(err))
	}

	return nil
}

func getImagesSearch(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	var (
		authEncoded = r.Header.Get("X-Registry-Auth")
		authConfig  = &auth.AuthConfig{}
		metaHeaders = map[string][]string{}
	)

	if authEncoded != "" {
		authJson := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJson).Decode(authConfig); err != nil {
			// for a search it is not an error if no auth was given
			// to increase compatibility with the existing api it is defaulting to be empty
			authConfig = &auth.AuthConfig{}
		}
	}
	for k, v := range r.Header {
		if strings.HasPrefix(k, "X-Meta-") {
			metaHeaders[k] = v
		}
	}

	var job = srv.Eng.Job("search", r.Form.Get("term"))
	job.SetenvJson("metaHeaders", metaHeaders)
	job.SetenvJson("authConfig", authConfig)
	job.Stdout.Add(w)

	return job.Run()
}

func postImagesInsert(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	if version > 1.0 {
		w.Header().Set("Content-Type", "application/json")
	}

	job := srv.Eng.Job("insert", vars["name"], r.Form.Get("url"), r.Form.Get("path"))
	job.SetenvBool("json", version > 1.0)
	job.Stdout.Add(w)
	if err := job.Run(); err != nil {
		if !job.Stdout.Used() {
			return err
		}
		sf := utils.NewStreamFormatter(version > 1.0)
		w.Write(sf.FormatError(err))
	}

	return nil
}

func postImagesPush(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

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

	if version > 1.0 {
		w.Header().Set("Content-Type", "application/json")
	}
	job := srv.Eng.Job("push", vars["name"])
	job.SetenvJson("metaHeaders", metaHeaders)
	job.SetenvJson("authConfig", authConfig)
	job.SetenvBool("json", version > 1.0)
	job.Stdout.Add(utils.NewWriteFlusher(w))

	if err := job.Run(); err != nil {
		if !job.Stdout.Used() {
			return err
		}
		sf := utils.NewStreamFormatter(version > 1.0)
		w.Write(sf.FormatError(err))
	}
	return nil
}

func getImagesGet(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	if version > 1.0 {
		w.Header().Set("Content-Type", "application/x-tar")
	}
	job := srv.Eng.Job("image_export", vars["name"])
	job.Stdout.Add(w)
	return job.Run()
}

func postImagesLoad(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	job := srv.Eng.Job("load")
	job.Stdin.Add(r.Body)
	return job.Run()
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
	if job.GetenvInt("Memory") > 0 && !srv.runtime.sysInfo.MemoryLimit {
		log.Println("WARNING: Your kernel does not support memory limit capabilities. Limitation discarded.")
		out.Warnings = append(out.Warnings, "Your kernel does not support memory limit capabilities. Limitation discarded.")
	}
	if job.GetenvInt("Memory") > 0 && !srv.runtime.sysInfo.SwapLimit {
		log.Println("WARNING: Your kernel does not support swap limit capabilities. Limitation discarded.")
		out.Warnings = append(out.Warnings, "Your kernel does not support memory swap capabilities. Limitation discarded.")
	}

	if !job.GetenvBool("NetworkDisabled") && srv.runtime.sysInfo.IPv4ForwardingDisabled {
		log.Println("Warning: IPv4 forwarding is disabled.")
		out.Warnings = append(out.Warnings, "IPv4 forwarding is disabled.")
	}

	return writeJSON(w, http.StatusCreated, out)
}

func postContainersRestart(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	job := srv.Eng.Job("restart", vars["name"])
	job.Setenv("t", r.Form.Get("t"))
	if err := job.Run(); err != nil {
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
	job := srv.Eng.Job("container_delete", vars["name"])
	job.Setenv("removeVolume", r.Form.Get("v"))
	job.Setenv("removeLink", r.Form.Get("link"))
	if err := job.Run(); err != nil {
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
	var job = srv.Eng.Job("image_delete", vars["name"])
	job.Stdout.Add(w)
	job.SetenvBool("autoPrune", version > 1.1)

	return job.Run()
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
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	job := srv.Eng.Job("stop", vars["name"])
	job.Setenv("t", r.Form.Get("t"))
	if err := job.Run(); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func postContainersWait(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	job := srv.Eng.Job("wait", vars["name"])
	var statusStr string
	job.Stdout.AddString(&statusStr)
	if err := job.Run(); err != nil {
		return err
	}
	// Parse a 16-bit encoded integer to map typical unix exit status.
	status, err := strconv.ParseInt(statusStr, 10, 16)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, &APIWait{StatusCode: int(status)})
}

func postContainersResize(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	if err := srv.Eng.Job("resize", vars["name"], r.Form.Get("h"), r.Form.Get("w")).Run(); err != nil {
		return err
	}
	return nil
}

func postContainersAttach(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	// TODO: replace the buffer by job.AddEnv()
	var (
		job    = srv.Eng.Job("inspect", vars["name"], "container")
		buffer = bytes.NewBuffer(nil)
		c      Container
	)
	job.Stdout.Add(buffer)
	if err := job.Run(); err != nil {
		return err
	}

	if err := json.Unmarshal(buffer.Bytes(), &c); err != nil {
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

	job = srv.Eng.Job("attach", vars["name"])
	job.Setenv("logs", r.Form.Get("logs"))
	job.Setenv("stream", r.Form.Get("stream"))
	job.Setenv("stdin", r.Form.Get("stdin"))
	job.Setenv("stdout", r.Form.Get("stdout"))
	job.Setenv("stderr", r.Form.Get("stderr"))
	job.Stdin.Add(inStream)
	job.Stdout.Add(outStream)
	job.Stderr.Set(errStream)
	if err := job.Run(); err != nil {
		fmt.Fprintf(outStream, "Error: %s\n", err)

	}
	return nil
}

func wsContainersAttach(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	if err := srv.Eng.Job("inspect", vars["name"], "container").Run(); err != nil {
		return err
	}

	h := websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		job := srv.Eng.Job("attach", vars["name"])
		job.Setenv("logs", r.Form.Get("logs"))
		job.Setenv("stream", r.Form.Get("stream"))
		job.Setenv("stdin", r.Form.Get("stdin"))
		job.Setenv("stdout", r.Form.Get("stdout"))
		job.Setenv("stderr", r.Form.Get("stderr"))
		job.Stdin.Add(ws)
		job.Stdout.Add(ws)
		job.Stderr.Set(ws)
		if err := job.Run(); err != nil {
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
	var job = srv.Eng.Job("inspect", vars["name"], "container")
	job.Stdout.Add(w)
	job.SetenvBool("conflict", true) //conflict=true to detect conflict between containers and images in the job
	return job.Run()
}

func getImagesByName(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	var job = srv.Eng.Job("inspect", vars["name"], "image")
	job.Stdout.Add(w)
	job.SetenvBool("conflict", true) //conflict=true to detect conflict between containers and images in the job
	return job.Run()
}

func postBuild(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if version < 1.3 {
		return fmt.Errorf("Multipart upload for build is no longer supported. Please upgrade your docker client.")
	}
	var (
		authEncoded       = r.Header.Get("X-Registry-Auth")
		authConfig        = &auth.AuthConfig{}
		configFileEncoded = r.Header.Get("X-Registry-Config")
		configFile        = &auth.ConfigFile{}
		job               = srv.Eng.Job("build")
	)

	// This block can be removed when API versions prior to 1.9 are deprecated.
	// Both headers will be parsed and sent along to the daemon, but if a non-empty
	// ConfigFile is present, any value provided as an AuthConfig directly will
	// be overridden. See BuildFile::CmdFrom for details.
	if version < 1.9 && authEncoded != "" {
		authJson := base64.NewDecoder(base64.URLEncoding, strings.NewReader(authEncoded))
		if err := json.NewDecoder(authJson).Decode(authConfig); err != nil {
			// for a pull it is not an error if no auth was given
			// to increase compatibility with the existing api it is defaulting to be empty
			authConfig = &auth.AuthConfig{}
		}
	}

	if configFileEncoded != "" {
		configFileJson := base64.NewDecoder(base64.URLEncoding, strings.NewReader(configFileEncoded))
		if err := json.NewDecoder(configFileJson).Decode(configFile); err != nil {
			// for a pull it is not an error if no auth was given
			// to increase compatibility with the existing api it is defaulting to be empty
			configFile = &auth.ConfigFile{}
		}
	}

	if version >= 1.8 {
		w.Header().Set("Content-Type", "application/json")
		job.SetenvBool("json", true)
	}

	job.Stdout.Add(utils.NewWriteFlusher(w))
	job.Stdin.Add(r.Body)
	job.Setenv("remote", r.FormValue("remote"))
	job.Setenv("t", r.FormValue("t"))
	job.Setenv("q", r.FormValue("q"))
	job.Setenv("nocache", r.FormValue("nocache"))
	job.Setenv("rm", r.FormValue("rm"))

	if err := job.Run(); err != nil {
		if !job.Stdout.Used() {
			return err
		}
		sf := utils.NewStreamFormatter(version >= 1.8)
		w.Write(sf.FormatError(err))
	}
	return nil
}

func postContainersCopy(srv *Server, version float64, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

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

	job := srv.Eng.Job("container_copy", vars["name"], copyData.Resource)
	job.Stdout.Add(w)
	if err := job.Run(); err != nil {
		utils.Errorf("%s", err.Error())
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
			http.Error(w, fmt.Errorf("client and server don't have same version (client : %g, server: %g)", version, APIVERSION).Error(), http.StatusNotFound)
			return
		}

		if err := handlerFunc(srv, version, w, r, mux.Vars(r)); err != nil {
			utils.Errorf("Error: %s", err)
			httpError(w, err)
		}
	}
}

// Replicated from expvar.go as not public.
func expvarHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, "{\n")
	first := true
	expvar.Do(func(kv expvar.KeyValue) {
		if !first {
			fmt.Fprintf(w, ",\n")
		}
		first = false
		fmt.Fprintf(w, "%q: %s", kv.Key, kv.Value)
	})
	fmt.Fprintf(w, "\n}\n")
}

func AttachProfiler(router *mux.Router) {
	router.HandleFunc("/debug/vars", expvarHandler)
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

// ServeFD creates an http.Server and sets it up to serve given a socket activated
// argument.
func ServeFd(addr string, handle http.Handler) error {
	ls, e := systemd.ListenFD(addr)
	if e != nil {
		return e
	}

	chErrors := make(chan error, len(ls))

	// Since ListenFD will return one or more sockets we have
	// to create a go func to spawn off multiple serves
	for i := range ls {
		listener := ls[i]
		go func() {
			httpSrv := http.Server{Handler: handle}
			chErrors <- httpSrv.Serve(listener)
		}()
	}

	for i := 0; i < len(ls); i += 1 {
		err := <-chErrors
		if err != nil {
			return err
		}
	}

	return nil
}

// ListenAndServe sets up the required http.Server and gets it listening for
// each addr passed in and does protocol specific checking.
func ListenAndServe(proto, addr string, srv *Server, logging bool) error {
	r, err := createRouter(srv, logging)
	if err != nil {
		return err
	}

	if proto == "fd" {
		return ServeFd(addr, r)
	}

	if proto == "unix" {
		if err := syscall.Unlink(addr); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	l, err := net.Listen(proto, addr)
	if err != nil {
		return err
	}

	// Basic error and sanity checking
	switch proto {
	case "tcp":
		if !strings.HasPrefix(addr, "127.0.0.1") {
			log.Println("/!\\ DON'T BIND ON ANOTHER IP ADDRESS THAN 127.0.0.1 IF YOU DON'T KNOW WHAT YOU'RE DOING /!\\")
		}
	case "unix":
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
	default:
		return fmt.Errorf("Invalid protocol format.")
	}

	httpSrv := http.Server{Addr: addr, Handler: r}
	return httpSrv.Serve(l)
}
