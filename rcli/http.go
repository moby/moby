package rcli

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
)

// Use this key to encode an RPC call into an URL,
// eg. domain.tld/path/to/method?q=get_user&q=gordon
const ARG_URL_KEY = "q"

func URLToCall(u *url.URL) (method string, args []string) {
	return path.Base(u.Path), u.Query()[ARG_URL_KEY]
}

func ListenAndServeHTTP(addr string, service Service) error {
	return http.ListenAndServe(addr, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			cmd, args := URLToCall(r.URL)
			if err := call(service, r.Body, &AutoFlush{w}, append([]string{cmd}, args...)...); err != nil {
				fmt.Fprintf(w, "Error: "+err.Error()+"\n")
			}
		}))
}

type AutoFlush struct {
	http.ResponseWriter
}

func (w *AutoFlush) Write(data []byte) (int, error) {
	ret, err := w.ResponseWriter.Write(data)
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
	return ret, err
}
