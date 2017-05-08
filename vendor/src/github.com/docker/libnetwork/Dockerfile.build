FROM golang:1.5.4
RUN apt-get update && apt-get -y install iptables

RUN go get github.com/tools/godep \
		github.com/golang/lint/golint \
		golang.org/x/tools/cmd/cover\
		github.com/mattn/goveralls
