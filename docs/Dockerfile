#
# See the top level Makefile in https://github.com/docker/docker for usage.
#
FROM debian:jessie
MAINTAINER Sven Dowideit <SvenDowideit@docker.com> (@SvenDowideit)

RUN apt-get update \
	&& apt-get install -y \
		gettext \
		git \
		libssl-dev \
		make \
		python-dev \
		python-pip \
		python-setuptools \
		vim-tiny

RUN pip install mkdocs

# add MarkdownTools to get transclusion
# (future development)
#RUN easy_install -U setuptools
#RUN pip install MarkdownTools2

# this version works, the current versions fail in different ways
RUN pip install awscli==1.4.4 pyopenssl==0.12

# get my sitemap.xml branch of mkdocs and use that for now
# commit hash of the newest commit of SvenDowideit/mkdocs on
# docker-markdown-merge branch, it is used to break docker cache
# see: https://github.com/SvenDowideit/mkdocs/tree/docker-markdown-merge
RUN git clone -b docker-markdown-merge https://github.com/SvenDowideit/mkdocs \
	&& cd mkdocs/ \
	&& git checkout ad32549c452963b8854951d6783f4736c0f7c5d5 \
	&& ./setup.py install

COPY . /docs
COPY MAINTAINERS /docs/sources/humans.txt
WORKDIR /docs

RUN VERSION=$(cat VERSION) \
	&& MAJOR_MINOR="${VERSION%.*}" \
	&& for i in $(seq $MAJOR_MINOR -0.1 1.0); do \
		echo "<li><a class='version' href='/v$i'>Version v$i</a></li>"; \
	done > sources/versions.html_fragment \
	&& GIT_BRANCH=$(cat GIT_BRANCH) \
	&& GITCOMMIT=$(cat GITCOMMIT) \
	&& AWS_S3_BUCKET=$(cat AWS_S3_BUCKET) \
	&& BUILD_DATE=$(date) \
	&& sed -i "s/\$VERSION/$VERSION/g" theme/mkdocs/base.html \
	&& sed -i "s/\$MAJOR_MINOR/v$MAJOR_MINOR/g" theme/mkdocs/base.html \
	&& sed -i "s/\$GITCOMMIT/$GITCOMMIT/g" theme/mkdocs/base.html \
	&& sed -i "s/\$GIT_BRANCH/$GIT_BRANCH/g" theme/mkdocs/base.html \
	&& sed -i "s/\$BUILD_DATE/$BUILD_DATE/g" theme/mkdocs/base.html \
	&& sed -i "s/\$AWS_S3_BUCKET/$AWS_S3_BUCKET/g" theme/mkdocs/base.html

EXPOSE 8000

CMD ["mkdocs", "serve"]
