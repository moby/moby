package builder

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

func TestCloneArgsSmartHttp(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	serverURL, _ := url.Parse(server.URL)

	serverURL.Path = "/repo.git"
	gitURL := serverURL.String()

	mux.HandleFunc("/repo.git/info/refs", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("service")
		w.Header().Set("Content-Type", fmt.Sprintf("application/x-%s-advertisement", q))
	})

	args := cloneArgs(gitURL, "/tmp")
	exp := []string{"clone", "--recursive", "--depth", "1", gitURL, "/tmp"}
	if !reflect.DeepEqual(args, exp) {
		t.Fatalf("Expected %v, got %v", exp, args)
	}
}

func TestCloneArgsDumbHttp(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	serverURL, _ := url.Parse(server.URL)

	serverURL.Path = "/repo.git"
	gitURL := serverURL.String()

	mux.HandleFunc("/repo.git/info/refs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
	})

	args := cloneArgs(gitURL, "/tmp")
	exp := []string{"clone", "--recursive", gitURL, "/tmp"}
	if !reflect.DeepEqual(args, exp) {
		t.Fatalf("Expected %v, got %v", exp, args)
	}
}
func TestCloneArgsGit(t *testing.T) {
	args := cloneArgs("git://github.com/docker/docker", "/tmp")
	exp := []string{"clone", "--recursive", "--depth", "1", "git://github.com/docker/docker", "/tmp"}
	if !reflect.DeepEqual(args, exp) {
		t.Fatalf("Expected %v, got %v", exp, args)
	}
}
