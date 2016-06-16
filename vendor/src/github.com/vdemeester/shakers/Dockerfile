FROM golang:1.5

RUN go get golang.org/x/tools/cmd/cover
RUN go get github.com/golang/lint/golint
RUN go get golang.org/x/tools/cmd/vet
RUN go get github.com/Masterminds/glide

WORKDIR /go/src/github.com/vdemeester/shakers

# enable GO15VENDOREXPERIMENT
ENV GO15VENDOREXPERIMENT 1

COPY glide.yaml glide.yaml
RUN glide up

COPY . /go/src/github.com/vdemeester/shakers
