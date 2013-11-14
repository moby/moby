default: build

build:
	sudo docker build -t docker .
	sudo docker run -privileged -v `pwd`:/go/src/github.com/dotcloud/docker docker hack/make.sh binary

doc:
	cd docs && docker build -t docker:docs . && docker run -p 8000:8000 docker:docs

test:
	sudo docker run -privileged -v `pwd`:/go/src/github.com/dotcloud/docker docker hack/make.sh test

shell:
	sudo docker run -privileged -i -t docker bash
