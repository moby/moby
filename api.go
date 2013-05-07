package docker

import (
	_ "bytes"
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/auth"
	"github.com/gorilla/mux"
	"github.com/shin-/cookiejar"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func hijackServer(w http.ResponseWriter) (*os.File, net.Conn, error) {
	rwc, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return nil, nil, err
	}

	file, err := rwc.(*net.TCPConn).File()
	if err != nil {
		return nil, rwc, err
	}

	// Flush the options to make sure the client sets the raw mode
	rwc.Write([]byte{})

	return file, rwc, nil
}

func httpError(w http.ResponseWriter, err error) {
	if strings.HasPrefix(err.Error(), "No such") {
		http.Error(w, err.Error(), http.StatusNotFound)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func getAuth(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	var out auth.AuthConfig
	out.Username = srv.runtime.authConfig.Username
	out.Email = srv.runtime.authConfig.Email
	b, err := json.Marshal(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
	return nil
}

func postAuth(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	var config auth.AuthConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	if config.Username == srv.runtime.authConfig.Username {
		config.Password = srv.runtime.authConfig.Password
	}

	newAuthConfig := auth.NewAuthConfig(config.Username, config.Password, config.Email, srv.runtime.root)
	status, err := auth.Login(newAuthConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	} else {
		srv.runtime.graph.getHttpClient().Jar = cookiejar.NewCookieJar()
		srv.runtime.authConfig = newAuthConfig
	}
	if status != "" {
		b, err := json.Marshal(ApiAuth{status})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		}
	} else {
		w.WriteHeader(http.StatusOK)
	}
	return nil
}

func getVersion(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	m := srv.DockerVersion()
	b, err := json.Marshal(m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
	return nil
}

func postContainersKill(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	vars := mux.Vars(r)
	name := vars["name"]
	if err := srv.ContainerKill(name); err != nil {
		httpError(w, err)
		return err
	} else {
		w.WriteHeader(http.StatusOK)
	}
	return nil
}

func getContainersExport(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	vars := mux.Vars(r)
	name := vars["name"]

	file, rwc, err := hijackServer(w)
	if file != nil {
		defer file.Close()
	}
	if rwc != nil {
		defer rwc.Close()
	}
	if err != nil {
		httpError(w, err)
		return err
	}
	fmt.Fprintf(file, "HTTP/1.1 200 OK\r\nContent-Type: raw-stream-hijack\r\n\r\n")
	if err := srv.ContainerExport(name, file); err != nil {
		fmt.Fprintf(file, "Error: %s\n", err)
		return err
	}
	return nil
}

func getImages(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	viz := r.Form.Get("viz")
	if viz == "1" {
		file, rwc, err := hijackServer(w)
		if file != nil {
			defer file.Close()
		}
		if rwc != nil {
			defer rwc.Close()
		}
		if err != nil {
			httpError(w, err)
			return err
		}
		fmt.Fprintf(file, "HTTP/1.1 200 OK\r\nContent-Type: raw-stream-hijack\r\n\r\n")
		if err := srv.ImagesViz(file); err != nil {
			fmt.Fprintf(file, "Error: %s\n", err)
		}
		return nil
	}

	all := r.Form.Get("all")
	filter := r.Form.Get("filter")
	quiet := r.Form.Get("quiet")

	outs, err := srv.Images(all, filter, quiet)
	if err != nil {
		httpError(w, err)
		return err
	}
	b, err := json.Marshal(outs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
	return nil
}

func getInfo(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	out := srv.DockerInfo()
	b, err := json.Marshal(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
	return nil
}

func getImagesHistory(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	vars := mux.Vars(r)
	name := vars["name"]
	outs, err := srv.ImageHistory(name)
	if err != nil {
		httpError(w, err)
		return err
	}
	b, err := json.Marshal(outs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
	return nil
}

func getContainersChanges(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	vars := mux.Vars(r)
	name := vars["name"]
	changesStr, err := srv.ContainerChanges(name)
	if err != nil {
		httpError(w, err)
		return err
	}
	b, err := json.Marshal(changesStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
	return nil
}

func getContainersPort(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	vars := mux.Vars(r)
	name := vars["name"]
	out, err := srv.ContainerPort(name, r.Form.Get("port"))
	if err != nil {
		httpError(w, err)
		return err
	}
	b, err := json.Marshal(ApiPort{out})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
	return nil
}

func getContainers(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	all := r.Form.Get("all")
	notrunc := r.Form.Get("notrunc")
	quiet := r.Form.Get("quiet")
	n, err := strconv.Atoi(r.Form.Get("n"))
	if err != nil {
		n = -1
	}

	outs := srv.Containers(all, notrunc, quiet, n)
	b, err := json.Marshal(outs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
	return nil
}

func postImagesTag(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	repo := r.Form.Get("repo")
	tag := r.Form.Get("tag")
	vars := mux.Vars(r)
	name := vars["name"]
	var force bool
	if r.Form.Get("force") == "1" {
		force = true
	}

	if err := srv.ContainerTag(name, repo, tag, force); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	w.WriteHeader(http.StatusCreated)
	return nil
}

func postCommit(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	var config Config
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	repo := r.Form.Get("repo")
	tag := r.Form.Get("tag")
	container := r.Form.Get("container")
	author := r.Form.Get("author")
	comment := r.Form.Get("comment")
	id, err := srv.ContainerCommit(container, repo, tag, author, comment, &config)
	if err != nil {
		httpError(w, err)
		return err
	}
	b, err := json.Marshal(ApiId{id})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
	return nil
}

func postImages(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	src := r.Form.Get("fromSrc")
	image := r.Form.Get("fromImage")
	repo := r.Form.Get("repo")
	tag := r.Form.Get("tag")

	file, rwc, err := hijackServer(w)
	if file != nil {
		defer file.Close()
	}
	if rwc != nil {
		defer rwc.Close()
	}
	if err != nil {
		httpError(w, err)
		return err
	}
	fmt.Fprintf(file, "HTTP/1.1 200 OK\r\nContent-Type: raw-stream-hijack\r\n\r\n")
	if image != "" { //pull
		registry := r.Form.Get("registry")
		if err := srv.ImagePull(image, tag, registry, file); err != nil {
			fmt.Fprintf(file, "Error: %s\n", err)
			return err
		}
	} else { //import
		if err := srv.ImageImport(src, repo, tag, file); err != nil {
			fmt.Fprintf(file, "Error: %s\n", err)
			return err
		}
	}
	return nil
}

func getImagesSearch(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	term := r.Form.Get("term")
	outs, err := srv.ImagesSearch(term)
	if err != nil {
		httpError(w, err)
		return err
	}
	b, err := json.Marshal(outs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
	return nil
}

func postImagesInsert(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	url := r.Form.Get("url")
	path := r.Form.Get("path")
	vars := mux.Vars(r)
	name := vars["name"]

	file, rwc, err := hijackServer(w)
	if file != nil {
		defer file.Close()
	}
	if rwc != nil {
		defer rwc.Close()
	}
	if err != nil {
		httpError(w, err)
		return err
	}
	fmt.Fprintf(file, "HTTP/1.1 200 OK\r\nContent-Type: raw-stream-hijack\r\n\r\n")
	if err := srv.ImageInsert(name, url, path, file); err != nil {
		fmt.Fprintln(file, "Error: "+err.Error())
	}
	return nil
}

func postImagesPush(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	registry := r.Form.Get("registry")

	vars := mux.Vars(r)
	name := vars["name"]

	file, rwc, err := hijackServer(w)
	if file != nil {
		defer file.Close()
	}
	if rwc != nil {
		defer rwc.Close()
	}
	if err != nil {
		httpError(w, err)
		return err
	}
	fmt.Fprintf(file, "HTTP/1.1 200 OK\r\nContent-Type: raw-stream-hijack\r\n\r\n")
	if err := srv.ImagePush(name, registry, file); err != nil {
		fmt.Fprintln(file, "Error: "+err.Error())
	}
	return nil
}

func postBuild(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)

	file, rwc, err := hijackServer(w)
	if file != nil {
		defer file.Close()
	}
	if rwc != nil {
		defer rwc.Close()
	}
	if err != nil {
		httpError(w, err)
		return err
	}
	fmt.Fprintf(file, "HTTP/1.1 200 OK\r\nContent-Type: raw-stream-hijack\r\n\r\n")
	if err := srv.ImageCreateFormFile(file); err != nil {
		fmt.Fprintln(file, "Error: "+err.Error())
	}
	return nil
}

func postContainers(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	var config Config
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	id, memoryW, swapW, err := srv.ContainerCreate(config)
	if err != nil {
		httpError(w, err)
		return err
	}
	var out ApiRun
	out.Id = id
	if memoryW {
		out.Warnings = append(out.Warnings, "Your kernel does not support memory limit capabilities. Limitation discarded.")
	}
	if swapW {
		out.Warnings = append(out.Warnings, "Your kernel does not support memory swap capabilities. Limitation discarded.")
	}
	b, err := json.Marshal(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
	return nil
}

func postContainersRestart(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	t, err := strconv.Atoi(r.Form.Get("t"))
	if err != nil || t < 0 {
		t = 10
	}
	vars := mux.Vars(r)
	name := vars["name"]
	if err := srv.ContainerRestart(name, t); err != nil {
		httpError(w, err)
		return err
	} else {
		w.WriteHeader(http.StatusOK)
	}
	return nil
}

func deleteContainers(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	vars := mux.Vars(r)
	name := vars["name"]
	var v bool
	if r.Form.Get("v") == "1" {
		v = true
	}
	if err := srv.ContainerDestroy(name, v); err != nil {
		httpError(w, err)
		return err
	} else {
		w.WriteHeader(http.StatusOK)
	}
	return nil
}

func deleteImages(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	vars := mux.Vars(r)
	name := vars["name"]
	if err := srv.ImageDelete(name); err != nil {
		httpError(w, err)
		return err
	} else {
		w.WriteHeader(http.StatusOK)
	}
	return nil
}

func postContainersStart(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	vars := mux.Vars(r)
	name := vars["name"]
	if err := srv.ContainerStart(name); err != nil {
		httpError(w, err)
		return err
	} else {
		w.WriteHeader(http.StatusOK)
	}
	return nil
}

func postContainersStop(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	t, err := strconv.Atoi(r.Form.Get("t"))
	if err != nil || t < 0 {
		t = 10
	}
	vars := mux.Vars(r)
	name := vars["name"]

	if err := srv.ContainerStop(name, t); err != nil {
		httpError(w, err)
		return err
	} else {
		w.WriteHeader(http.StatusOK)
	}
	return nil
}

func postContainersWait(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	vars := mux.Vars(r)
	name := vars["name"]
	status, err := srv.ContainerWait(name)
	if err != nil {
		httpError(w, err)
		return err
	}
	b, err := json.Marshal(ApiWait{status})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
	return nil
}

func postContainersAttach(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	logs := r.Form.Get("logs")
	stream := r.Form.Get("stream")
	stdin := r.Form.Get("stdin")
	stdout := r.Form.Get("stdout")
	stderr := r.Form.Get("stderr")
	vars := mux.Vars(r)
	name := vars["name"]

	file, rwc, err := hijackServer(w)
	if file != nil {
		defer file.Close()
	}
	if rwc != nil {
		defer rwc.Close()
	}
	if err != nil {
		httpError(w, err)
		return err
	}

	fmt.Fprintf(file, "HTTP/1.1 200 OK\r\nContent-Type: raw-stream-hijack\r\n\r\n")
	if err := srv.ContainerAttach(name, logs, stream, stdin, stdout, stderr, file); err != nil {
		fmt.Fprintf(file, "Error: %s\n", err)
		return err
	}
	return nil
}

func getContainersByName(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	vars := mux.Vars(r)
	name := vars["name"]

	container, err := srv.ContainerInspect(name)
	if err != nil {
		httpError(w, err)
		return err
	}
	b, err := json.Marshal(container)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
	return nil
}

func getImagesByName(srv *Server, w http.ResponseWriter, r *http.Request) error {
	log.Println(r.Method, r.RequestURI)
	vars := mux.Vars(r)
	name := vars["name"]

	image, err := srv.ImageInspect(name)
	if err != nil {
		httpError(w, err)
		return err
	}
	b, err := json.Marshal(image)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
	return nil
}

func wrap(fct func(*Server, http.ResponseWriter, *http.Request) error, w http.ResponseWriter, r *http.Request, srv *Server, method, route string) {
	if err := fct(srv, w, r); err != nil {
		Debugf("Error: %s %s: %s", method, route, err)
	}
}

func ListenAndServe(addr string, srv *Server) error {
	r := mux.NewRouter()
	log.Printf("Listening for HTTP on %s\n", addr)

	m := map[string]map[string]func(*Server, http.ResponseWriter, *http.Request) error{
		"GET": {
			"/auth":                         getAuth,
			"/version":                      getVersion,
			"/containers/{name:.*}/export":  getContainersExport,
			"/images":                       getImages,
			"/info":                         getInfo,
			"/images/{name:.*}/history":     getImagesHistory,
			"/containers/{name:.*}/changes": getContainersChanges,
			"/containers/{name:.*}/port":    getContainersPort,
			"/containers":                   getContainers,
			"/images/search":                getImagesSearch,
			"/containers/{name:.*}":         getContainersByName,
			"/images/{name:.*}":             getImagesByName,
		},
		"POST": {
			"/auth": postAuth,
			"/containers/{name:.*}/kill":    postContainersKill,
			"/images/{name:.*}/tag":         postImagesTag,
			"/commit":                       postCommit,
			"/images":                       postImages,
			"/images/{name:*.}/insert":      postImagesInsert,
			"/images/{name:*.}/push":        postImagesPush,
			"/postBuild":                    postBuild,
			"/postContainers":               postContainers,
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
			Debugf("Registering %s, %s", method, route)
			// NOTE: scope issue, make sure the variables are local and won't be changed
			localRoute := route
			localMethod := method
			localFct := fct
			r.Path(localRoute).Methods(localMethod).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Debugf("Calling %s %s", localMethod, localRoute)
				localFct(srv, w, r)
			})
		}
	}

	return http.ListenAndServe(addr, r)
}
