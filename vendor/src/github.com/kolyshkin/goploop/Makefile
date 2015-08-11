CGO_LDFLAGS	:= -l:libploop.a
export CGO_LDFLAGS

all: build

build:
	go build -v

test:
	go test -v .

.PHONY: all build test
