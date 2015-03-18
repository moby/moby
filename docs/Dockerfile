#
# See the top level Makefile in https://github.com/docker/docker for usage.
#
FROM docs/base:latest
MAINTAINER Sven Dowideit <SvenDowideit@docker.com> (@SvenDowideit)

# TODO: need the full repo source to get the git version info
COPY . /src

# Reset the /docs dir so we can replace the theme meta with the new repo's git info
RUN git reset --hard

# Then copy the desired docs into the /docs/sources/ dir
COPY ./sources/ /docs/sources

COPY ./VERSION VERSION

# adding the image spec will require Docker 1.5 and `docker build -f docs/Dockerfile .`
#COPY ./image/spec/v1.md /docs/sources/reference/image-spec-v1.md

# TODO: don't do this - look at merging the yml file in build.sh
COPY ./mkdocs.yml mkdocs.yml
COPY ./s3_website.json s3_website.json
COPY ./release.sh release.sh

# Docker Swarm
#ADD https://raw.githubusercontent.com/docker/swarm/master/docs/mkdocs.yml /docs/mkdocs-swarm.yml
ADD https://raw.githubusercontent.com/docker/swarm/master/docs/index.md /docs/sources/swarm/index.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/swarm/index.md
ADD https://raw.githubusercontent.com/docker/swarm/master/discovery/README.md /docs/sources/swarm/discovery.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/swarm/discovery.md
ADD https://raw.githubusercontent.com/docker/swarm/master/api/README.md /docs/sources/swarm/API.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/swarm/API.md
ADD https://raw.githubusercontent.com/docker/swarm/master/scheduler/filter/README.md /docs/sources/swarm/scheduler/filter.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/swarm/scheduler/filter.md
ADD https://raw.githubusercontent.com/docker/swarm/master/scheduler/strategy/README.md /docs/sources/swarm/scheduler/strategy.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/swarm/scheduler/strategy.md

# Docker Machine
#ADD https://raw.githubusercontent.com/docker/machine/master/docs/mkdocs.yml /docs/mkdocs-machine.yml
ADD https://raw.githubusercontent.com/docker/machine/master/docs/index.md /docs/sources/machine/index.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/machine/index.md

# Docker Compose
#ADD https://raw.githubusercontent.com/docker/compose/master/docs/mkdocs.yml /docs/mkdocs-compose.yml
ADD https://raw.githubusercontent.com/docker/compose/master/docs/index.md /docs/sources/compose/index.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/index.md
ADD https://raw.githubusercontent.com/docker/compose/master/docs/install.md /docs/sources/compose/install.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/install.md
ADD https://raw.githubusercontent.com/docker/compose/master/docs/cli.md /docs/sources/compose/cli.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/cli.md
ADD https://raw.githubusercontent.com/docker/compose/master/docs/yml.md /docs/sources/compose/yml.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/yml.md
ADD https://raw.githubusercontent.com/docker/compose/master/docs/env.md /docs/sources/compose/env.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/env.md
ADD https://raw.githubusercontent.com/docker/compose/master/docs/completion.md /docs/sources/compose/completion.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/completion.md

ADD https://raw.githubusercontent.com/docker/compose/master/docs/django.md /docs/sources/compose/django.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/django.md
ADD https://raw.githubusercontent.com/docker/compose/master/docs/rails.md /docs/sources/compose/rails.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/rails.md
ADD https://raw.githubusercontent.com/docker/compose/master/docs/wordpress.md /docs/sources/compose/wordpress.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/wordpress.md

# Then build everything together, ready for mkdocs
RUN /docs/build.sh
