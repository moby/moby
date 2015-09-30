package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/runconfig"
	"golang.org/x/net/context"
)

// @Title getExecByID
// @Description Get exec process within the container by exec ID
// @Param   version     path    string     false        "API version number"
// @Param   id          path    string     true         "Execution ID"
// @Success 200 {object} daemon.ExecConfig
// @SubApi /exec
// @Router /exec/:id/json [get]
func (s *Server) getExecByID(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if vars == nil {
		return fmt.Errorf("Missing parameter 'id'")
	}

	eConfig, err := s.daemon.ContainerExecInspect(vars["id"])
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, eConfig)
}

// @Title postContainerExecCreate
// @Description Create a new execution process inside the container
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container ID or name"
// @Param   execConfig  body    []byte     true         "Execution configuration"
// @Success 201 {object} types.ContainerExecCreateResponse
// @SubApi /containers
// @Router /containers/:name/exec [post]
func (s *Server) postContainerExecCreate(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if err := checkForJSON(r); err != nil {
		return err
	}
	name := vars["name"]

	execConfig := &runconfig.ExecConfig{}
	if err := json.NewDecoder(r.Body).Decode(execConfig); err != nil {
		return err
	}
	execConfig.Container = name

	if len(execConfig.Cmd) == 0 {
		return fmt.Errorf("No exec command specified")
	}

	// Register an instance of Exec in container.
	id, err := s.daemon.ContainerExecCreate(execConfig)
	if err != nil {
		logrus.Errorf("Error setting up exec command in container %s: %s", name, err)
		return err
	}

	return writeJSON(w, http.StatusCreated, &types.ContainerExecCreateResponse{
		ID: id,
	})
}

// @Title postContainerExecStart
// @Description Start an execution process inside the container
// @Param   version         path    string     false        "API version number"
// @Param   name            path    string     true         "Container ID or name"
// @Param   execStartCheck  body    []byte     true         "Execution configuration"
// @Success 200
// @SubApi /exec
// @Router /exec/:name/start [post]
func (s *Server) postContainerExecStart(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	var (
		execName                  = vars["name"]
		stdin, inStream           io.ReadCloser
		stdout, stderr, outStream io.Writer
	)

	execStartCheck := &types.ExecStartCheck{}
	if err := json.NewDecoder(r.Body).Decode(execStartCheck); err != nil {
		return err
	}

	if !execStartCheck.Detach {
		var err error
		// Setting up the streaming http interface.
		inStream, outStream, err = hijackServer(w)
		if err != nil {
			return err
		}
		defer closeStreams(inStream, outStream)

		if _, ok := r.Header["Upgrade"]; ok {
			fmt.Fprintf(outStream, "HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n")
		} else {
			fmt.Fprintf(outStream, "HTTP/1.1 200 OK\r\nContent-Type: application/vnd.docker.raw-stream\r\n\r\n")
		}

		stdin = inStream
		stdout = outStream
		if !execStartCheck.Tty {
			stderr = stdcopy.NewStdWriter(outStream, stdcopy.Stderr)
			stdout = stdcopy.NewStdWriter(outStream, stdcopy.Stdout)
		}
	} else {
		outStream = w
	}

	// Now run the user process in container.
	if err := s.daemon.ContainerExecStart(execName, stdin, stdout, stderr); err != nil {
		fmt.Fprintf(outStream, "Error running exec in container: %v\n", err)
	}
	return nil
}

// @Title postContainerExecResize
// @Description Resize tty for the execution process
// @Param   version     path    string     false        "API version number"
// @Param   name        path    string     true         "Container ID or name"
// @Param   h           query   integer    false        "High of the tty"
// @Param   w           query   integer    false        "Width of the tty"
// @Success 200
// @SubApi /exec
// @Router /exec/:name/resize [post]
func (s *Server) postContainerExecResize(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := parseForm(r); err != nil {
		return err
	}
	if vars == nil {
		return fmt.Errorf("Missing parameter")
	}

	height, err := strconv.Atoi(r.Form.Get("h"))
	if err != nil {
		return err
	}
	width, err := strconv.Atoi(r.Form.Get("w"))
	if err != nil {
		return err
	}

	return s.daemon.ContainerExecResize(vars["name"], height, width)
}
