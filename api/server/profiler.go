package server

import (
	"expvar"
	"fmt"
	"net/http"
	"net/http/pprof"

	"github.com/gorilla/mux"
)

func NewProfiler() http.Handler {
	var (
		p = &Profiler{}
		r = mux.NewRouter()
	)
	r.HandleFunc("/vars", p.expVars)
	r.HandleFunc("/pprof/", pprof.Index)
	r.HandleFunc("/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/pprof/profile", pprof.Profile)
	r.HandleFunc("/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/pprof/block", pprof.Handler("block").ServeHTTP)
	r.HandleFunc("/pprof/heap", pprof.Handler("heap").ServeHTTP)
	r.HandleFunc("/pprof/goroutine", pprof.Handler("goroutine").ServeHTTP)
	r.HandleFunc("/pprof/threadcreate", pprof.Handler("threadcreate").ServeHTTP)
	p.r = r
	return p
}

// Profiler enables pprof and expvar support via a HTTP API.
type Profiler struct {
	r *mux.Router
}

func (p *Profiler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.r.ServeHTTP(w, r)
}

// Replicated from expvar.go as not public.
func (p *Profiler) expVars(w http.ResponseWriter, r *http.Request) {
	first := true
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, "{\n")
	expvar.Do(func(kv expvar.KeyValue) {
		if !first {
			fmt.Fprintf(w, ",\n")
		}
		first = false
		fmt.Fprintf(w, "%q: %s", kv.Key, kv.Value)
	})
	fmt.Fprintf(w, "\n}\n")
}
