default: test

testdeps:
	@go get github.com/onsi/ginkgo
	@go get github.com/onsi/gomega

test: testdeps
	@go test ./...

testrace: testdeps
	@go test ./... -race

testall: test testrace
