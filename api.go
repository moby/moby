package docker

import (
	"encoding/json"
	"log"
	"github.com/gorilla/mux"
	"net/http"
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

	r.Path("/images").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//TODO use runtime
	})

	return http.ListenAndServe(addr, r)
}

