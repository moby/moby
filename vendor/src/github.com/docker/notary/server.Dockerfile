FROM golang:1.5.3
MAINTAINER David Lawrence "david.lawrence@docker.com"

RUN apt-get update && apt-get install -y \
    libltdl-dev \
    --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*

EXPOSE 4443

# Install DB migration tool
RUN go get github.com/mattes/migrate

ENV NOTARYPKG github.com/docker/notary
ENV GOPATH /go/src/${NOTARYPKG}/Godeps/_workspace:$GOPATH



# Copy the local repo to the expected go path
COPY . /go/src/github.com/docker/notary

WORKDIR /go/src/${NOTARYPKG}

# Install notary-server
RUN go install \
    -tags pkcs11 \
    -ldflags "-w -X ${NOTARYPKG}/version.GitCommit=`git rev-parse --short HEAD` -X ${NOTARYPKG}/version.NotaryVersion=`cat NOTARY_VERSION`" \
    ${NOTARYPKG}/cmd/notary-server

ENTRYPOINT [ "notary-server" ]
CMD [ "-config=fixtures/server-config-local.json" ]
