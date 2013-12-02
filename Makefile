default: build

build: bundles
	docker build -t docker .
	docker run -privileged -v `pwd`/bundles:/go/src/github.com/dotcloud/docker/bundles docker hack/make.sh binary

doc:
	cd docs && docker build -t docker-docs . && docker run -p 8000:8000 docker-docs

test: bundles
	docker run -privileged -v `pwd`/bundles:/go/src/github.com/dotcloud/docker/bundles docker hack/make.sh test

shell:
	docker run -privileged -i -t docker bash

bundles:
	mkdir bundles

