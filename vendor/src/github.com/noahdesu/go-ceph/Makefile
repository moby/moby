DOCKER_CI_IMAGE = go-ceph-ci
build:
	go build -v
fmt:
	go fmt ./...
test:
	go test -v ./...

test-docker: .build-docker
	docker run --rm -it -v $(CURDIR):/go/src/github.com/noahdesu/go-ceph $(DOCKER_CI_IMAGE)

.build-docker:
	docker build -t $(DOCKER_CI_IMAGE) .
	@docker inspect -f '{{.Id}}' $(DOCKER_CI_IMAGE) > .build-docker
