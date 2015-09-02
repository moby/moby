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
	"github.com/docker/docker/pkg/version"
	"github.com/docker/docker/runconfig"
	restful "github.com/emicklei/go-restful"
)

func (s *Server) getExecByID(version version.Version, w *restful.Response, r *restful.Request) error {
	eConfig, err := s.daemon.ContainerExecInspect(r.PathParameter("id"))
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, eConfig)
}

func (s *Server) postContainerExecCreate(version version.Version, w *restful.Response, r *restful.Request) error {
	if err := parseForm(r.Request); err != nil {
		return err
	}
	if err := checkForJSON(r.Request); err != nil {
		return err
	}
	name := r.PathParameter("name")

	execConfig := &runconfig.ExecConfig{}
	if err := json.NewDecoder(r.Request.Body).Decode(execConfig); err != nil {
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

// TODO(vishh): Refactor the code to avoid having to specify stream config as part of both create and start.
func (s *Server) postContainerExecStart(version version.Version, w *restful.Response, r *restful.Request) error {
	if err := parseForm(r.Request); err != nil {
		return err
	}
	var (
		execName                  = r.PathParameter("name")
		stdin, inStream           io.ReadCloser
		stdout, stderr, outStream io.Writer
	)

	execStartCheck := &types.ExecStartCheck{}
	if err := json.NewDecoder(r.Request.Body).Decode(execStartCheck); err != nil {
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

		if h := r.HeaderParameter("Upgrade"); h != "" {
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

func (s *Server) postContainerExecResize(version version.Version, w *restful.Response, r *restful.Request) error {
	if err := parseForm(r.Request); err != nil {
		return err
	}

	height, err := strconv.Atoi(r.Request.Form.Get("h"))
	if err != nil {
		return err
	}
	width, err := strconv.Atoi(r.Request.Form.Get("w"))
	if err != nil {
		return err
	}

	return s.daemon.ContainerExecResize(r.PathParameter("name"), height, width)
}
