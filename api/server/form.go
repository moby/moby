package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func boolValue(r *http.Request, k string) bool {
	s := strings.ToLower(strings.TrimSpace(r.FormValue(k)))
	return !(s == "" || s == "0" || s == "no" || s == "false" || s == "none")
}

// boolValueOrDefault returns the default bool passed if the query param is
// missing, otherwise it's just a proxy to boolValue above
func boolValueOrDefault(r *http.Request, k string, d bool) bool {
	if _, ok := r.Form[k]; !ok {
		return d
	}
	return boolValue(r, k)
}

func int64ValueOrZero(r *http.Request, k string) int64 {
	val, err := strconv.ParseInt(r.FormValue(k), 10, 64)
	if err != nil {
		return 0
	}
	return val
}

type archiveOptions struct {
	name string
	path string
}

func archiveFormValues(r *http.Request, vars map[string]string) (archiveOptions, error) {
	if vars == nil {
		return archiveOptions{}, fmt.Errorf("Missing parameter")
	}
	if err := parseForm(r); err != nil {
		return archiveOptions{}, err
	}

	name := vars["name"]
	path := r.Form.Get("path")

	switch {
	case name == "":
		return archiveOptions{}, fmt.Errorf("bad parameter: 'name' cannot be empty")
	case path == "":
		return archiveOptions{}, fmt.Errorf("bad parameter: 'path' cannot be empty")
	}

	return archiveOptions{name, path}, nil
}
