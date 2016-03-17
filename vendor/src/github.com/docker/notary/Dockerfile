FROM golang:1.6.0

RUN apt-get update && apt-get install -y \
	libltdl-dev \
	libsqlite3-dev \
	--no-install-recommends \
	&& rm -rf /var/lib/apt/lists/*

RUN go get golang.org/x/tools/cmd/vet \
	&& go get golang.org/x/tools/cmd/cover \
	&& go get github.com/tools/godep

COPY . /go/src/github.com/docker/notary

WORKDIR /go/src/github.com/docker/notary
