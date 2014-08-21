#
# See the top level Makefile in https://github.com/docker/docker for usage.
#
FROM 		debian:jessie
MAINTAINER	Sven Dowideit <SvenDowideit@docker.com> (@SvenDowideit)

RUN 	apt-get update && apt-get install -y make python-pip python-setuptools vim-tiny git gettext

RUN	pip install mkdocs

# add MarkdownTools to get transclusion
# (future development)
#RUN	easy_install -U setuptools
#RUN	pip install MarkdownTools2

# this version works, the current versions fail in different ways
RUN	pip install awscli==1.3.9

# make sure the git clone is not an old cache - we've published old versions a few times now
ENV	CACHE_BUST Jul2014

# get my sitemap.xml branch of mkdocs and use that for now
RUN	git clone https://github.com/SvenDowideit/mkdocs	&&\
	cd mkdocs/						&&\
	git checkout docker-markdown-merge			&&\
	./setup.py install

ADD 	. /docs
ADD	MAINTAINERS /docs/sources/humans.txt
WORKDIR	/docs

RUN	VERSION=$(cat /docs/VERSION)								&&\
        MAJOR_MINOR="${VERSION%.*}"								&&\
	for i in $(seq $MAJOR_MINOR -0.1 1.0) ; do echo "<li><a class='version' href='/v$i'>Version v$i</a></li>" ; done > /docs/sources/versions.html_fragment &&\
	GIT_BRANCH=$(cat /docs/GIT_BRANCH)							&&\
	GITCOMMIT=$(cat /docs/GITCOMMIT)							&&\
	AWS_S3_BUCKET=$(cat /docs/AWS_S3_BUCKET)						&&\
	BUILD_DATE=$(date)									&&\
	sed -i "s/\$VERSION/$VERSION/g" /docs/theme/mkdocs/base.html				&&\
	sed -i "s/\$MAJOR_MINOR/v$MAJOR_MINOR/g" /docs/theme/mkdocs/base.html			&&\
	sed -i "s/\$GITCOMMIT/$GITCOMMIT/g" /docs/theme/mkdocs/base.html			&&\
	sed -i "s/\$GIT_BRANCH/$GIT_BRANCH/g" /docs/theme/mkdocs/base.html			&&\
	sed -i "s/\$BUILD_DATE/$BUILD_DATE/g" /docs/theme/mkdocs/base.html				&&\
	sed -i "s/\$AWS_S3_BUCKET/$AWS_S3_BUCKET/g" /docs/theme/mkdocs/base.html

# note, EXPOSE is only last because of https://github.com/docker/docker/issues/3525
EXPOSE	8000

CMD 	["mkdocs", "serve"]
