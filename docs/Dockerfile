FROM 		debian
MAINTAINER	SvenDowideit@docker.com
#
#    docker build -t docker:docs . && docker run -p 8000:8000 docker:docs
#

RUN apt-get update && apt-get install -yq make python-pip python-setuptools

RUN	pip install mkdocs

ADD 	. /docs
WORKDIR	/docs

CMD 	["mkdocs", "serve"]

# note, EXPOSE is only last because of https://github.com/dotcloud/docker/issues/3525
EXPOSE	8000
