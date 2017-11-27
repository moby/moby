## Step 1: Build tests
FROM golang:1.9.2-alpine3.6 as builder

RUN apk add --update \
    bash \
    btrfs-progs-dev \
    build-base \
    curl \
    lvm2-dev \
    jq \
    && rm -rf /var/cache/apk/*

RUN mkdir -p /go/src/github.com/docker/docker/
WORKDIR /go/src/github.com/docker/docker/

# Generate frozen images
COPY contrib/download-frozen-image-v2.sh contrib/download-frozen-image-v2.sh
RUN contrib/download-frozen-image-v2.sh /output/docker-frozen-images \
  buildpack-deps:jessie@sha256:85b379ec16065e4fe4127eb1c5fb1bcc03c559bd36dbb2e22ff496de55925fa6 \
  busybox:latest@sha256:32f093055929dbc23dec4d03e09dfe971f5973a9ca5cf059cbfb644c206aa83f \
  debian:jessie@sha256:72f784399fd2719b4cb4e16ef8e369a39dc67f53d978cd3e2e7bf4e502c7b793 \
  hello-world:latest@sha256:c5515758d4c5e1e838e9cd307f6c6a0d620b5e07e6f927b07d05f6d12a1ac8d7

# Download Docker CLI binary
COPY hack/dockerfile hack/dockerfile
RUN hack/dockerfile/install-binaries.sh dockercli

# Set tag and add sources
ARG DOCKER_GITCOMMIT
ENV DOCKER_GITCOMMIT=$DOCKER_GITCOMMIT
ADD . .

# Build DockerSuite.TestBuild* dependency
RUN CGO_ENABLED=0 go build -o /output/httpserver github.com/docker/docker/contrib/httpserver

# Build the integration tests and copy the resulting binaries to /output/tests
RUN hack/make.sh build-integration-test-binary
RUN mkdir -p /output/tests && find . -name test.main -exec cp --parents '{}' /output/tests \;

## Step 2: Generate testing image
FROM alpine:3.6 as runner

# GNU tar is used for generating the emptyfs image
RUN apk add --update \
    bash \
    ca-certificates \
    g++ \
    git \
    iptables \
    tar \
    xz \
    && rm -rf /var/cache/apk/*

# Add an unprivileged user to be used for tests which need it
RUN addgroup docker && adduser -D -G docker unprivilegeduser -s /bin/ash

COPY contrib/httpserver/Dockerfile /tests/contrib/httpserver/Dockerfile
COPY contrib/syscall-test /tests/contrib/syscall-test
COPY integration-cli/fixtures /tests/integration-cli/fixtures

COPY hack/test/e2e-run.sh /scripts/run.sh
COPY hack/make/.ensure-emptyfs /scripts/ensure-emptyfs.sh

COPY --from=builder /output/docker-frozen-images /docker-frozen-images
COPY --from=builder /output/httpserver /tests/contrib/httpserver/httpserver
COPY --from=builder /output/tests /tests
COPY --from=builder /usr/local/bin/docker /usr/bin/docker

ENV DOCKER_REMOTE_DAEMON=1 DOCKER_INTEGRATION_DAEMON_DEST=/

ENTRYPOINT ["/scripts/run.sh"]
