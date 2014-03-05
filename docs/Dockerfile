FROM 		debian:stable
MAINTAINER	SvenDowideit@docker.com
#
#    docker build -t docker:docs . && docker run -p 8000:8000 docker:docs
#

RUN 	apt-get update && apt-get install -yq make python-pip python-setuptools
RUN	pip install mkdocs

RUN	apt-get install -yq vim-tiny git pandoc

# pip installs from docs/requirements.txt, but here to increase cacheability
RUN	pip install Sphinx==1.2.1
RUN	pip install sphinxcontrib-httpdomain==1.2.0

# add MarkdownTools to get transclusion
RUN	easy_install -U setuptools
RUN	pip install MarkdownTools2

ADD 	. /docs

#build the sphinx html
RUN	make -C /docs clean docs

WORKDIR	/docs

#CMD 	["mkdocs", "serve"]
CMD	bash

# note, EXPOSE is only last because of https://github.com/dotcloud/docker/issues/3525
EXPOSE	8000
