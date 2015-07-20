package client

import (
	"net/http"
)

// GetCookieJar returns a cookie jar for the client library to use.  It is part
// of an interface which the http client looks for in the list of objects that
// we pass to its SetAuth() method.
func (cli *DockerCli) GetCookieJar() http.CookieJar {
	return cli.jar
}
