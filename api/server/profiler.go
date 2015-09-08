package server

import (
	"expvar"
	"fmt"
	"net/http"
	"net/http/pprof"

	restful "github.com/emicklei/go-restful"
)

func profilerRouter(path string) *restful.WebService {
	ws := new(restful.WebService)
	ws.Path(path)

	ws.Route(ws.GET("/vars").To(restfulHTTPRoute(expVars)))
	ws.Route(ws.GET("/pprof/").To(restfulHTTPRoute(pprof.Index)))
	ws.Route(ws.GET("/pprof/cmdline").To(restfulHTTPRoute(pprof.Cmdline)))
	ws.Route(ws.GET("/pprof/profile").To(restfulHTTPRoute(pprof.Profile)))
	ws.Route(ws.GET("/pprof/symbol").To(restfulHTTPRoute(pprof.Symbol)))
	ws.Route(ws.GET("/pprof/block").To(restfulHTTPRoute(pprof.Handler("block").ServeHTTP)))
	ws.Route(ws.GET("/pprof/heap").To(restfulHTTPRoute(pprof.Handler("heap").ServeHTTP)))
	ws.Route(ws.GET("/pprof/goroutine").To(restfulHTTPRoute(pprof.Handler("goroutine").ServeHTTP)))
	ws.Route(ws.GET("/pprof/threadcreate").To(restfulHTTPRoute(pprof.Handler("threadcreate").ServeHTTP)))

	return ws
}

func restfulHTTPRoute(handler func(w http.ResponseWriter, r *http.Request)) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		handler(resp.ResponseWriter, req.Request)
	}
}

// Replicated from expvar.go as not public.
func expVars(w http.ResponseWriter, r *http.Request) {
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
