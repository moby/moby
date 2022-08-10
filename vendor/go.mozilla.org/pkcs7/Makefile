all: vet staticcheck test

test:
	go test -covermode=count -coverprofile=coverage.out .

showcoverage: test
	go tool cover -html=coverage.out

vet:
	go vet .

lint:
	golint .

staticcheck:
	staticcheck .

gettools:
	go get -u honnef.co/go/tools/...
	go get -u golang.org/x/lint/golint
