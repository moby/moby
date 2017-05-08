FROM golang:1.6.1-alpine
MAINTAINER David Lawrence "david.lawrence@docker.com"

RUN apk add --update git gcc libc-dev && rm -rf /var/cache/apk/*

# Install SQL DB migration tool
RUN go get github.com/mattes/migrate

ENV NOTARYPKG github.com/docker/notary

# Copy the local repo to the expected go path
COPY . /go/src/${NOTARYPKG}

WORKDIR /go/src/${NOTARYPKG}

EXPOSE 4443

# Install notary-server
RUN go install \
    -tags pkcs11 \
    -ldflags "-w -X ${NOTARYPKG}/version.GitCommit=`git rev-parse --short HEAD` -X ${NOTARYPKG}/version.NotaryVersion=`cat NOTARY_VERSION`" \
    ${NOTARYPKG}/cmd/notary-server && apk del git gcc libc-dev

ENTRYPOINT [ "notary-server" ]
CMD [ "-config=fixtures/server-config-local.json" ]
