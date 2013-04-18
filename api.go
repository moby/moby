package docker

import (
	"encoding/json"
	_ "fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"
)

func ListenAndServe(addr string, runtime *Runtime) error {
	r := mux.NewRouter()
	log.Printf("Listening for HTTP on %s\n", addr)

	r.Path("/version").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := VersionOut{VERSION, GIT_COMMIT, NO_MEMORY_LIMIT}
		b, err := json.Marshal(m)
		if err != nil {
			w.WriteHeader(500)
		} else {
			w.Write(b)
		}
	})

	r.Path("/kill").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ids []string
		if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
			w.WriteHeader(500)
			return
		}

		var ret SimpleMessage
		for _, name := range ids {
			container := runtime.Get(name)
			if container == nil {
				ret.Message = "No such container: " + name + "\n"
				break
			}
			if err := container.Kill(); err != nil {
				ret.Message = ret.Message + "Error killing container " + name + ": " + err.Error() + "\n"
			}
		}
		if ret.Message == "" {
			w.WriteHeader(200)
		} else {
			w.WriteHeader(500)
		}

		b, err := json.Marshal(ret)
		if err != nil {
			w.WriteHeader(500)
		} else {
			w.Write(b)
		}

	})

	r.Path("/images").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in ImagesIn
		json.NewDecoder(r.Body).Decode(&in)

		var allImages map[string]*Image
		var err error
		if in.All {
			allImages, err = runtime.graph.Map()
		} else {
			allImages, err = runtime.graph.Heads()
		}
		if err != nil {
			w.WriteHeader(500)
			return
		}
		var outs []ImagesOut
		for name, repository := range runtime.repositories.Repositories {
			if in.NameFilter != "" && name != in.NameFilter {
				continue
			}
			for tag, id := range repository {
				var out ImagesOut
				image, err := runtime.graph.Get(id)
				if err != nil {
					log.Printf("Warning: couldn't load %s from %s/%s: %s", id, name, tag, err)
					continue
				}
				delete(allImages, id)
				if !in.Quiet {
					out.Repository = name
					out.Tag = tag
					out.Id = TruncateId(id)
					out.Created = HumanDuration(time.Now().Sub(image.Created)) + " ago"
				} else {
					out.Id = image.ShortId()
				}
				outs = append(outs, out)
			}
		}
		// Display images which aren't part of a
		if in.NameFilter == "" {
			for id, image := range allImages {
				var out ImagesOut
				if !in.Quiet {
					out.Repository = "<none>"
					out.Tag = "<none>"
					out.Id = TruncateId(id)
					out.Created = HumanDuration(time.Now().Sub(image.Created)) + " ago"
				} else {
					out.Id = image.ShortId()
				}
				outs = append(outs, out)
			}
		}

		b, err := json.Marshal(outs)
		if err != nil {
			w.WriteHeader(500)
		} else {
			w.Write(b)
		}

	})

	return http.ListenAndServe(addr, r)
}
