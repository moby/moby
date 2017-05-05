package server

import (
	"expvar"
	"fmt"
	"net/http"
	"net/http/pprof"

	"github.com/gorilla/mux"
)

const debugPathPrefix = "/debug/"

func profilerSetup(mainRouter *mux.Router) {
	var r = mainRouter.PathPrefix(debugPathPrefix).Subrouter()
	r.HandleFunc("/vars", expVars)
	r.HandleFunc("/pprof/", pprof.Index)
	r.HandleFunc("/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/pprof/profile", pprof.Profile)
	r.HandleFunc("/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/pprof/trace", pprof.Trace)
	r.HandleFunc("/pprof/{name}", handlePprof)
}

func handlePprof(w http.ResponseWriter, r *http.Request) {
	var name string
	if vars := mux.Vars(r); vars != nil {
		name = vars["name"]
	}
	pprof.Handler(name).ServeHTTP(w, r)
}

// Replicated from expvar.go as not public.
func expVars(w http.ResponseWriter, r *http.Request) {
	first := true
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintln(w, "{")
	expvar.Do(func(kv expvar.KeyValue) {
		if !first {
			fmt.Fprintln(w, ",")
		}
		first = false
		fmt.Fprintf(w, "%q: %s", kv.Key, kv.Value)
	})
	fmt.Fprintln(w, "\n}")
}
