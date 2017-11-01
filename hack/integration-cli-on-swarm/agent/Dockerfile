# this Dockerfile is solely used for the master image.
# Please refer to the top-level Makefile for the worker image.
FROM golang:1.7
ADD . /go/src/github.com/moby/moby/hack/integration-cli-on-swarm/agent
RUN go build -o /master github.com/moby/moby/hack/integration-cli-on-swarm/agent/master
ENTRYPOINT ["/master"]
