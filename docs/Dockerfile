#
# See the top level Makefile in https://github.com/dotcloud/docker for usage.
#
FROM 		debian:testing
MAINTAINER	Sven Dowideit <SvenDowideit@docker.com> (@SvenDowideit)

RUN 	apt-get update && apt-get install -yq make gcc python-sphinx python-pip python-setuptools
RUN	pip install mkdocs

RUN	apt-get install -yq vim-tiny git pandoc s3cmd=1.1.0*

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


# note, EXPOSE is only last because of https://github.com/dotcloud/docker/issues/3525
EXPOSE	8000

# Setup s3cmd config
ADD	s3cfg  /.s3cfg

CMD 	["mkdocs", "serve"]
