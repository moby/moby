#
# See the top level Makefile in https://github.com/docker/docker for usage.
#
FROM docs/base:latest
MAINTAINER Sven Dowideit <SvenDowideit@docker.com> (@SvenDowideit)

# This section ensures we pull the correct version of each
# sub project
ENV COMPOSE_BRANCH release
ENV SWARM_BRANCH v0.2.0
ENV MACHINE_BRANCH docs
ENV DISTRIB_BRANCH docs


# TODO: need the full repo source to get the git version info
COPY . /src

# Reset the /docs dir so we can replace the theme meta with the new repo's git info
# RUN git reset --hard

# Then copy the desired docs into the /docs/sources/ dir
COPY ./sources/ /docs/sources

COPY ./VERSION VERSION

# adding the image spec will require Docker 1.5 and `docker build -f docs/Dockerfile .`
#COPY ./image/spec/v1.md /docs/sources/reference/image-spec-v1.md

# TODO: don't do this - look at merging the yml file in build.sh
COPY ./mkdocs.yml ./s3_website.json ./release.sh ./

#######################
# Docker Distribution
########################

#ADD https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/mkdocs.yml /docs/mkdocs-distribution.yml

ADD https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/images/notifications.png \
    https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/images/registry.png \
  /docs/sources/registry/images/

ADD https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/index.md \
    https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/deploying.md \
    https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/configuration.md \
    https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/storagedrivers.md \
    https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/notifications.md \
  /docs/sources/registry/

ADD https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/spec/api.md \
    https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/spec/json.md \
  /docs/sources/registry/spec/
  
ADD https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/storage-drivers/s3.md \
    https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/storage-drivers/azure.md \
    https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/storage-drivers/filesystem.md \
    https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/storage-drivers/inmemory.md \
  /docs/sources/registry/storage-drivers/

ADD https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/spec/auth/token.md /docs/sources/registry/spec/auth/token.md

RUN sed -i.old '1s;^;no_version_dropdown: true;' \
  /docs/sources/registry/*.md \
  /docs/sources/registry/spec/*.md \
  /docs/sources/registry/spec/auth/*.md \
  /docs/sources/registry/storage-drivers/*.md 

RUN sed -i.old  -e '/^<!--GITHUB/g' -e '/^IGNORES-->/g'\
  /docs/sources/registry/*.md \
  /docs/sources/registry/spec/*.md \
  /docs/sources/registry/spec/auth/*.md \
  /docs/sources/registry/storage-drivers/*.md 

#######################
# Docker Swarm
#######################

#ADD https://raw.githubusercontent.com/docker/swarm/${SWARM_BRANCH}/docs/mkdocs.yml /docs/mkdocs-swarm.yml
ADD https://raw.githubusercontent.com/docker/swarm/${SWARM_BRANCH}/docs/index.md /docs/sources/swarm/index.md

ADD https://raw.githubusercontent.com/docker/swarm/${SWARM_BRANCH}/discovery/README.md /docs/sources/swarm/discovery.md

ADD https://raw.githubusercontent.com/docker/swarm/${SWARM_BRANCH}/api/README.md /docs/sources/swarm/API.md

ADD https://raw.githubusercontent.com/docker/swarm/${SWARM_BRANCH}/scheduler/filter/README.md /docs/sources/swarm/scheduler/filter.md

ADD https://raw.githubusercontent.com/docker/swarm/${SWARM_BRANCH}/scheduler/strategy/README.md /docs/sources/swarm/scheduler/strategy.md

RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/swarm/*.md /docs/sources/swarm/scheduler/*.md

#######################
# Docker Machine
#######################
#ADD https://raw.githubusercontent.com/docker/machine/${MACHINE_BRANCH}/docs/mkdocs.yml /docs/mkdocs-machine.yml

ADD https://raw.githubusercontent.com/docker/machine/${MACHINE_BRANCH}/docs/index.md /docs/sources/machine/index.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/machine/index.md

#######################
# Docker Compose
#######################

#ADD https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/mkdocs.yml /docs/mkdocs-compose.yml

ADD https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/index.md \
  https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/install.md \
  https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/cli.md \
  https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/yml.md \
  https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/env.md \
  https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/completion.md \
  https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/django.md \
  https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/rails.md \
  https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/wordpress.md \
  https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/extends.md \
  https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/production.md \
  /docs/sources/compose/

RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/*.md

# Then build everything together, ready for mkdocs
RUN /docs/build.sh
