#
# See the top level Makefile in https://github.com/docker/docker for usage.
#
FROM docs/base:latest
MAINTAINER Sven Dowideit <SvenDowideit@docker.com> (@SvenDowideit)

# This section ensures we pull the correct version of each
# sub project
ENV COMPOSE_BRANCH 1.2.0
ENV SWARM_BRANCH v0.2.0
ENV MACHINE_BRANCH master
ENV DISTRIB_BRANCH v2.0.0



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
COPY ./mkdocs.yml mkdocs.yml
COPY ./s3_website.json s3_website.json
COPY ./release.sh release.sh


# Docker Distribution
# 
#ADD https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/mkdocs.yml /docs/mkdocs-distribution.yml

ADD https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/images/notifications.png /docs/sources/registry/images/notifications.png
ADD https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/images/registry.png /docs/sources/registry/images/registry.png

ADD https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/overview.md /docs/sources/registry/overview.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/registry/overview.md

ADD https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/deploying.md /docs/sources/registry/deploying.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/registry/deploying.md

ADD https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/configuration.md /docs/sources/registry/configuration.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/registry/configuration.md

ADD https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/storagedrivers.md /docs/sources/registry/storagedrivers.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/registry/storagedrivers.md

ADD https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/notifications.md /docs/sources/registry/notifications.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/registry/notifications.md

ADD https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/spec/api.md /docs/sources/registry/spec/api.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/registry/spec/api.md

ADD https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/spec/json.md /docs/sources/registry/spec/json.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/registry/spec/json.md

ADD https://raw.githubusercontent.com/docker/distribution/${DISTRIB_BRANCH}/docs/spec/auth/token.md /docs/sources/registry/spec/auth/token.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/registry/spec/auth/token.md

# Docker Swarm
#ADD https://raw.githubusercontent.com/docker/swarm/${SWARM_BRANCH}/docs/mkdocs.yml /docs/mkdocs-swarm.yml
ADD https://raw.githubusercontent.com/docker/swarm/${SWARM_BRANCH}/docs/index.md /docs/sources/swarm/index.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/swarm/index.md
ADD https://raw.githubusercontent.com/docker/swarm/${SWARM_BRANCH}/discovery/README.md /docs/sources/swarm/discovery.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/swarm/discovery.md
ADD https://raw.githubusercontent.com/docker/swarm/${SWARM_BRANCH}/api/README.md /docs/sources/swarm/API.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/swarm/API.md
ADD https://raw.githubusercontent.com/docker/swarm/${SWARM_BRANCH}/scheduler/filter/README.md /docs/sources/swarm/scheduler/filter.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/swarm/scheduler/filter.md
ADD https://raw.githubusercontent.com/docker/swarm/${SWARM_BRANCH}/scheduler/strategy/README.md /docs/sources/swarm/scheduler/strategy.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/swarm/scheduler/strategy.md

# Docker Machine
#ADD https://raw.githubusercontent.com/docker/machine/${MACHINE_BRANCH}/docs/mkdocs.yml /docs/mkdocs-machine.yml
ADD https://raw.githubusercontent.com/docker/machine/${MACHINE_BRANCH}/docs/index.md /docs/sources/machine/index.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/machine/index.md

# Docker Compose
#ADD https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/mkdocs.yml /docs/mkdocs-compose.yml
ADD https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/index.md /docs/sources/compose/index.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/index.md
ADD https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/install.md /docs/sources/compose/install.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/install.md
ADD https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/cli.md /docs/sources/compose/cli.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/cli.md
ADD https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/yml.md /docs/sources/compose/yml.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/yml.md
ADD https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/env.md /docs/sources/compose/env.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/env.md
ADD https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/completion.md /docs/sources/compose/completion.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/completion.md

ADD https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/django.md /docs/sources/compose/django.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/django.md
ADD https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/rails.md /docs/sources/compose/rails.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/rails.md
ADD https://raw.githubusercontent.com/docker/compose/${COMPOSE_BRANCH}/docs/wordpress.md /docs/sources/compose/wordpress.md
RUN sed -i.old '1s;^;no_version_dropdown: true;' /docs/sources/compose/wordpress.md

# Then build everything together, ready for mkdocs
RUN /docs/build.sh