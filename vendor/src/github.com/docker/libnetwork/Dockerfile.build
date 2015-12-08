FROM golang:1.4-cross
RUN apt-get update && apt-get -y install iptables
RUN go get github.com/tools/godep \
		github.com/golang/lint/golint \
		golang.org/x/tools/cmd/vet \
		golang.org/x/tools/cmd/goimports \
		golang.org/x/tools/cmd/cover\
		github.com/mattn/goveralls
