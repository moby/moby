//go:build !windows
// +build !windows

/*
 * Copyright (c) 2023. Nydus Developers. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package converter

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/containerd/containerd/content"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type contentStoreProxy struct {
	socketPath string
	server     *http.Server
}

func setupContentStoreProxy(workDir string, ra content.ReaderAt) (*contentStoreProxy, error) {
	sockP, err := os.CreateTemp(workDir, "nydus-cs-proxy-*.sock")
	if err != nil {
		return nil, errors.Wrap(err, "create unix socket file")
	}
	if err := os.Remove(sockP.Name()); err != nil {
		return nil, err
	}
	listener, err := net.Listen("unix", sockP.Name())
	if err != nil {
		return nil, errors.Wrap(err, "listen unix socket when setup content store proxy")
	}

	server := &http.Server{
		Handler: contentProxyHandler(ra),
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Warn("serve content store proxy")
		}
	}()

	return &contentStoreProxy{
		socketPath: sockP.Name(),
		server:     server,
	}, nil
}

func (p *contentStoreProxy) close() error {
	defer os.Remove(p.socketPath)
	if err := p.server.Shutdown(context.Background()); err != nil {
		return errors.Wrap(err, "shutdown content store proxy")
	}
	return nil
}

func parseRangeHeader(rangeStr string, totalLen int64) (start, wantedLen int64, err error) {
	rangeList := strings.Split(rangeStr, "-")
	start, err = strconv.ParseInt(rangeList[0], 10, 64)
	if err != nil {
		err = errors.Wrap(err, "parse range header")
		return
	}
	if len(rangeList) == 2 {
		var end int64
		end, err = strconv.ParseInt(rangeList[1], 10, 64)
		if err != nil {
			err = errors.Wrap(err, "parse range header")
			return
		}
		wantedLen = end - start + 1
	} else {
		wantedLen = totalLen - start
	}
	if start < 0 || start >= totalLen || wantedLen <= 0 {
		err = fmt.Errorf("invalid range header: %s", rangeStr)
		return
	}
	return
}

func contentProxyHandler(ra content.ReaderAt) http.Handler {
	var (
		dataReader io.Reader
		curPos     int64

		tarHeader *tar.Header
		totalLen  int64
	)
	resetReader := func() {
		// TODO: Handle error?
		_, _ = seekFile(ra, EntryBlob, func(reader io.Reader, hdr *tar.Header) error {
			dataReader, tarHeader = reader, hdr
			return nil
		})
		curPos = 0
	}

	resetReader()
	if tarHeader != nil {
		totalLen = tarHeader.Size
	} else {
		totalLen = ra.Size()
	}
	handler := func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			{
				w.Header().Set("Content-Length", strconv.FormatInt(totalLen, 10))
				w.Header().Set("Content-Type", "application/octet-stream")
				return
			}
		case http.MethodGet:
			{
				start, wantedLen, err := parseRangeHeader(strings.TrimPrefix(r.Header.Get("Range"), "bytes="), totalLen)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					// TODO: Handle error?
					_, _ = w.Write([]byte(err.Error()))
					return
				}

				// we need to make sure that the dataReader is at the right position
				if start < curPos {
					resetReader()
				}
				if start > curPos {
					_, err = io.CopyN(io.Discard, dataReader, start-curPos)
					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
						// TODO: Handle error?
						_, _ = w.Write([]byte(err.Error()))
						return
					}
					curPos = start
				}
				// then, the curPos must be equal to start

				readLen, err := io.CopyN(w, dataReader, wantedLen)
				if err != nil && !errors.Is(err, io.EOF) {
					w.WriteHeader(http.StatusInternalServerError)
					// TODO: Handle error?
					_, _ = w.Write([]byte(err.Error()))
					return
				}
				curPos += readLen
				w.Header().Set("Content-Length", strconv.FormatInt(readLen, 10))
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, start+readLen-1, totalLen))
				w.Header().Set("Content-Type", "application/octet-stream")
				return
			}
		}
	}
	return http.HandlerFunc(handler)
}
