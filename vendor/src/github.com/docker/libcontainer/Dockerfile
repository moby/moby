FROM crosbymichael/golang

RUN apt-get update && apt-get install -y gcc
RUN go get code.google.com/p/go.tools/cmd/cover

# setup a playground for us to spawn containers in
RUN mkdir /busybox && \
    curl -sSL 'https://github.com/jpetazzo/docker-busybox/raw/buildroot-2014.02/rootfs.tar' | tar -xC /busybox

RUN curl -sSL https://raw.githubusercontent.com/docker/docker/master/hack/dind -o /dind && \
    chmod +x /dind

COPY . /go/src/github.com/docker/libcontainer
WORKDIR /go/src/github.com/docker/libcontainer
RUN cp sample_configs/minimal.json /busybox/container.json

RUN go get -d -v ./...
RUN go install -v ./...

ENTRYPOINT ["/dind"]
CMD ["go", "test", "-cover", "./..."]
