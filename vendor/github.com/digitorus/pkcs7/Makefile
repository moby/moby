all: vet staticcheck test

test:
	GODEBUG=x509sha1=1 go test -covermode=count -coverprofile=coverage.out .

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
