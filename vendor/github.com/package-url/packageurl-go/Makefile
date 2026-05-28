.PHONY: test clean lint

test:
	go test -v -cover ./...

lint:
	go get -u golang.org/x/lint/golint
	golint -set_exit_status
