package client

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/term"
)

// GetCookieJar returns a cookie jar for the client library to use.  It is part
// of an interface which the http client looks for in the list of objects that
// we pass to its SetAuth() method.
func (cli *DockerCli) GetCookieJar() http.CookieJar {
	return cli.jar
}

// GetBasicAuth prompts for a user name and password for doing Basic
// authentication to the passed-in "realm".  It is part of the
// authn.BasicAuther interface which the http client looks for in the list of
// objects that we pass to its SetAuth() method.
func (cli *DockerCli) GetBasicAuth(realm string) (string, string, error) {
	username, ok := cli.authnOpts["basic.username"]
	if !ok {
		username = os.Getenv("DOCKER_AUTHN_USERID")
	}
	password := os.Getenv("DOCKER_AUTHN_PASSWORD")
	interactive, ok := cli.authnOpts["interactive"]
	if !ok {
		interactive = "true"
	}

	if username != "" && password != "" {
		return username, password, nil
	}

	if !cli.isTerminalIn || !cli.isTerminalOut {
		cli.Debug("not connected to a terminal, not prompting for Basic auth creds")
		return "", "", nil
	}
	if prompt, err := strconv.ParseBool(interactive); !prompt || err != nil {
		cli.Debug("interactive prompting disabled, not prompting for Basic auth creds")
		return "", "", nil
	}

	readInput := func(in io.Reader, out io.Writer) string {
		reader := bufio.NewReader(in)
		line, _, err := reader.ReadLine()
		if err != nil {
			fmt.Fprintln(out, err.Error())
			os.Exit(1)
		}
		return string(line)
	}

	if realm != "" {
		if username != "" {
			fmt.Fprintf(cli.out, "Username for %s [%s]: ", realm, username)
		} else {
			fmt.Fprintf(cli.out, "Username for %s: ", realm)
		}
	} else {
		if username != "" {
			fmt.Fprintf(cli.out, "Username[%s]: ", username)
		} else {
			fmt.Fprintf(cli.out, "Username: ")
		}
	}
	readUsername := strings.Trim(readInput(cli.in, cli.out), " ")
	if readUsername != "" {
		username = readUsername
	}
	if username == "" {
		return "", "", errors.New("user name required")
	}

	oldState, err := term.SaveState(cli.inFd)
	if err != nil {
		return "", "", err
	}
	fmt.Fprintf(cli.out, "Password: ")
	term.DisableEcho(cli.inFd, oldState)

	password = readInput(cli.in, cli.out)
	fmt.Fprint(cli.out, "\n")

	term.RestoreTerminal(cli.inFd, oldState)
	if password == "" {
		return "", "", errors.New("password required")
	}

	return username, password, nil
}

// GetBearerAuth checks for a token passed in either via an environment
// variable or a command-line option, and passes it back to the library.  It is
// part of the authn.BearerAuther interface which the http client looks for in
// the list of objects that we pass to its SetAuth() method.
func (cli *DockerCli) GetBearerAuth(challenge string) (string, error) {
	token, ok := cli.authnOpts["bearer.token"]
	if !ok {
		token = os.Getenv("DOCKER_BEARER_TOKEN")
	}
	if token == "" {
		return "", errors.New("token required")
	}
	return token, nil
}
