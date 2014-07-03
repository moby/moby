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

# this version works, the current versions fail in different ways
RUN	pip install awscli==1.3.9

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
	GITCOMMIT=$(cat /docs/GITCOMMIT)							&&\
	AWS_S3_BUCKET=$(cat /docs/AWS_S3_BUCKET)						&&\
	sed -i "s/\$VERSION/$VERSION/g" /docs/theme/mkdocs/base.html				&&\
	sed -i "s/\$GITCOMMIT/$GITCOMMIT/g" /docs/theme/mkdocs/base.html			&&\
	sed -i "s/\$GIT_BRANCH/$GIT_BRANCH/g" /docs/theme/mkdocs/base.html			&&\
	sed -i "s/\$AWS_S3_BUCKET/$AWS_S3_BUCKET/g" /docs/theme/mkdocs/base.html

# note, EXPOSE is only last because of https://github.com/dotcloud/docker/issues/3525
EXPOSE	8000

CMD 	["mkdocs", "serve"]
