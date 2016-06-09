FROM    golang:1.6.3-alpine

RUN     apk add -U git bash curl gcc musl-dev

RUN     export GLIDE=0.10.2; \
        export SRC=https://github.com/Masterminds/glide/releases/download/; \
        curl -sL ${SRC}/${GLIDE}/glide-${GLIDE}-linux-amd64.tar.gz | \
        tar -xz linux-amd64/glide && \
        mv linux-amd64/glide /usr/bin/glide && \
        chmod +x /usr/bin/glide

COPY    man/glide.yaml /manvendor/
COPY    man/glide.lock /manvendor/
WORKDIR /manvendor/
RUN     glide install && mv vendor src
ENV     GOPATH=$GOPATH:/go/src/github.com/docker/docker/vendor:/manvendor
RUN     go build -o /usr/bin/go-md2man github.com/cpuguy83/go-md2man

WORKDIR /go/src/github.com/docker/docker/
ENTRYPOINT ["man/generate.sh"]
