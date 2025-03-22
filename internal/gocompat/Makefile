.PHONY: verify
verify: generate
	GO111MODULE=on go test -v

.PHONY: generate
generate: clean
	GO111MODULE=off go generate .
	GO111MODULE=on go mod tidy

.PHONY: clean
clean:
	@rm -f go.mod go.sum main.go main_test.go
