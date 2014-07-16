FROM crosbymichael/golang

RUN apt-get update && apt-get install -y gcc

ADD . /go/src/github.com/docker/libcontainer
RUN cd /go/src/github.com/docker/libcontainer && go get -d ./... && go install ./...

CMD ["nsinit"]
