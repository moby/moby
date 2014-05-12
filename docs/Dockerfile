#
# See the top level Makefile in https://github.com/dotcloud/docker for usage.
#
FROM 		debian:jessie
MAINTAINER	Sven Dowideit <SvenDowideit@docker.com> (@SvenDowideit)

RUN 	apt-get update && apt-get install -yq make python-pip python-setuptools vim-tiny git gettext

RUN	pip install mkdocs

# add MarkdownTools to get transclusion
# (future development)
#RUN	easy_install -U setuptools
#RUN	pip install MarkdownTools2

# this week I seem to need the latest dev release of awscli too
# awscli 1.3.6 does --error-document correctly
# https://github.com/aws/aws-cli/commit/edc2290e173dfaedc70b48cfa3624d58c533c6c3
RUN	pip install awscli

# get my sitemap.xml branch of mkdocs and use that for now
RUN	git clone https://github.com/SvenDowideit/mkdocs	&&\
	cd mkdocs/						&&\
	git checkout docker-markdown-merge			&&\
	./setup.py install

ADD 	. /docs
ADD	MAINTAINERS /docs/sources/humans.txt
WORKDIR	/docs

RUN	VERSION=$(cat /docs/VERSION)								&&\
	GIT_BRANCH=$(cat /docs/GIT_BRANCH)							&&\
	AWS_S3_BUCKET=$(cat /docs/AWS_S3_BUCKET)						&&\
	echo "{% set docker_version = \"${VERSION}\" %}{% set docker_branch = \"${GIT_BRANCH}\" %}{% set aws_bucket = \"${AWS_S3_BUCKET}\" %}{% include \"beta_warning.html\" %}" > /docs/theme/mkdocs/version.html

# note, EXPOSE is only last because of https://github.com/dotcloud/docker/issues/3525
EXPOSE	8000

CMD 	["mkdocs", "serve"]
