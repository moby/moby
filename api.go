package docker

import (
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/auth"
	"github.com/dotcloud/docker/utils"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

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

func httpError(w http.ResponseWriter, err error) {
	if strings.HasPrefix(err.Error(), "No such") {
		http.Error(w, err.Error(), http.StatusNotFound)
	} else if strings.HasPrefix(err.Error(), "Bad parameter") {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeJson(w http.ResponseWriter, b []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func getBoolParam(value string) (bool, error) {
	if value == "1" || strings.ToLower(value) == "true" {
		return true, nil
	}
	if value == "" || value == "0" || strings.ToLower(value) == "false" {
		return false, nil
	}
	return false, fmt.Errorf("Bad parameter")
}

func getAuth(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	b, err := json.Marshal(srv.registry.GetAuthConfig())
	if err != nil {
		return err
	}
	writeJson(w, b)
	return nil
}

func postAuth(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	config := &auth.AuthConfig{}
	if err := json.NewDecoder(r.Body).Decode(config); err != nil {
		return err
	}

	if config.Username == srv.registry.GetAuthConfig().Username {
		config.Password = srv.registry.GetAuthConfig().Password
	}

	newAuthConfig := auth.NewAuthConfig(config.Username, config.Password, config.Email, srv.runtime.root)
	status, err := auth.Login(newAuthConfig)
	if err != nil {
		return err
	}
	srv.registry.ResetClient(newAuthConfig)

	if status != "" {
		b, err := json.Marshal(&ApiAuth{Status: status})
		if err != nil {
			return err
		}
		writeJson(w, b)
		return nil
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func getVersion(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	m := srv.DockerVersion()
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	writeJson(w, b)
	return nil
}

func postContainersKill(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

func getContainersExport(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]

	if err := srv.ContainerExport(name, w); err != nil {
		utils.Debugf("%s", err.Error())
		return err
	}
	return nil
}

func getImagesJson(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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
	writeJson(w, b)
	return nil
}

func getImagesViz(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := srv.ImagesViz(w); err != nil {
		return err
	}
	return nil
}

func getInfo(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	out := srv.DockerInfo()
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	writeJson(w, b)
	return nil
}

func getImagesHistory(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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
	writeJson(w, b)
	return nil
}

func getContainersChanges(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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
	writeJson(w, b)
	return nil
}

func getContainersPs(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	all, err := getBoolParam(r.Form.Get("all"))
	if err != nil {
		return err
	}
	since := r.Form.Get("since")
	before := r.Form.Get("before")
	n, err := strconv.Atoi(r.Form.Get("limit"))
	if err != nil {
		n = -1
	}

	outs := srv.Containers(all, n, since, before)
	b, err := json.Marshal(outs)
	if err != nil {
		return err
	}
	writeJson(w, b)
	return nil
}

func postImagesTag(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

func postCommit(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	config := &Config{}
	if err := json.NewDecoder(r.Body).Decode(config); err != nil {
		utils.Debugf("%s", err.Error())
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
	b, err := json.Marshal(&ApiId{id})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusCreated)
	writeJson(w, b)
	return nil
}

// Creates an image from Pull or from Import
func postImagesCreate(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	src := r.Form.Get("fromSrc")
	image := r.Form.Get("fromImage")
	tag := r.Form.Get("tag")
	repo := r.Form.Get("repo")

	if image != "" { //pull
		registry := r.Form.Get("registry")
		if err := srv.ImagePull(image, tag, registry, w); err != nil {
			return err
		}
	} else { //import
		if err := srv.ImageImport(src, repo, tag, r.Body, w); err != nil {
			return err
		}
	}
	return nil
}

func getImagesSearch(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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
	writeJson(w, b)
	return nil
}

func postImagesInsert(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}

	url := r.Form.Get("url")
	path := r.Form.Get("path")
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]

	if err := srv.ImageInsert(name, url, path, w); err != nil {
		return err
	}
	return nil
}

func postImagesPush(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	registry := r.Form.Get("registry")

	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]

	if err := srv.ImagePush(name, registry, w); err != nil {
		return err
	}
	return nil
}

func postContainersCreate(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	config := &Config{}
	if err := json.NewDecoder(r.Body).Decode(config); err != nil {
		return err
	}
	id, err := srv.ContainerCreate(config)
	if err != nil {
		return err
	}

	out := &ApiRun{
		Id: id,
	}
	if config.Memory > 0 && !srv.runtime.capabilities.MemoryLimit {
		log.Println("WARNING: Your kernel does not support memory limit capabilities. Limitation discarded.")
		out.Warnings = append(out.Warnings, "Your kernel does not support memory limit capabilities. Limitation discarded.")
	}
	if config.Memory > 0 && !srv.runtime.capabilities.SwapLimit {
		log.Println("WARNING: Your kernel does not support swap limit capabilities. Limitation discarded.")
		out.Warnings = append(out.Warnings, "Your kernel does not support memory swap capabilities. Limitation discarded.")
	}
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusCreated)
	writeJson(w, b)
	return nil
}

func postContainersRestart(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

func deleteContainers(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

func deleteImages(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]
	if err := srv.ImageDelete(name); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func postContainersStart(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]
	if err := srv.ContainerStart(name); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func postContainersStop(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

func postContainersWait(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}
	name := vars["name"]
	status, err := srv.ContainerWait(name)
	if err != nil {
		return err
	}
	b, err := json.Marshal(&ApiWait{StatusCode: status})
	if err != nil {
		return err
	}
	writeJson(w, b)
	return nil
}

func postContainersAttach(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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

	in, out, err := hijackServer(w)
	if err != nil {
		return err
	}
	defer in.Close()

	fmt.Fprintf(out, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
	if err := srv.ContainerAttach(name, logs, stream, stdin, stdout, stderr, in, out); err != nil {
		fmt.Fprintf(out, "Error: %s\n", err)
	}
	return nil
}

func getContainersByName(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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
	writeJson(w, b)
	return nil
}

func getImagesByName(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
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
	writeJson(w, b)
	return nil
}

func postImagesGetCache(srv *Server, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	apiConfig := &ApiImageConfig{}
	if err := json.NewDecoder(r.Body).Decode(apiConfig); err != nil {
		return err
	}

	image, err := srv.ImageGetCached(apiConfig.Id, apiConfig.Config)
	if err != nil {
		return err
	}
	if image == nil {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	apiId := &ApiId{Id: image.Id}
	b, err := json.Marshal(apiId)
	if err != nil {
		return err
	}
	writeJson(w, b)
	return nil
}

func ListenAndServe(addr string, srv *Server, logging bool) error {
	r := mux.NewRouter()
	log.Printf("Listening for HTTP on %s\n", addr)

	m := map[string]map[string]func(*Server, http.ResponseWriter, *http.Request, map[string]string) error{
		"GET": {
			"/auth":                         getAuth,
			"/version":                      getVersion,
			"/info":                         getInfo,
			"/images/json":                  getImagesJson,
			"/images/viz":                   getImagesViz,
			"/images/search":                getImagesSearch,
			"/images/{name:.*}/history":     getImagesHistory,
			"/images/{name:.*}/json":        getImagesByName,
			"/containers/ps":                getContainersPs,
			"/containers/{name:.*}/export":  getContainersExport,
			"/containers/{name:.*}/changes": getContainersChanges,
			"/containers/{name:.*}/json":    getContainersByName,
		},
		"POST": {
			"/auth":                         postAuth,
			"/commit":                       postCommit,
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
			"/containers/{name:.*}/attach":  postContainersAttach,
		},
		"DELETE": {
			"/containers/{name:.*}": deleteContainers,
			"/images/{name:.*}":     deleteImages,
		},
	}

	for method, routes := range m {
		for route, fct := range routes {
			utils.Debugf("Registering %s, %s", method, route)
			// NOTE: scope issue, make sure the variables are local and won't be changed
			localRoute := route
			localMethod := method
			localFct := fct
			r.Path(localRoute).Methods(localMethod).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				if err := localFct(srv, w, r, mux.Vars(r)); err != nil {
					httpError(w, err)
				}
			})
		}
	}

	return http.ListenAndServe(addr, r)
}
