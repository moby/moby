=========
docker-ci
=========

This directory contains docker-ci continuous integration system.
As expected, it is a fully dockerized and deployed using
docker-container-runner.
docker-ci is based on Buildbot, a continuous integration system designed
to automate the build/test cycle. By automatically rebuilding and testing
the tree each time something has changed, build problems are pinpointed
quickly, before other developers are inconvenienced by the failure.
We are running buildbot at Rackspace to verify docker and docker-registry
pass tests, and check for coverage code details.

docker-ci instance is at https://docker-ci.docker.io/waterfall

Inside docker-ci container we have the following directory structure:

/docker-ci                       source code of docker-ci
/data/backup/docker-ci/          daily backup (replicated over S3)
/data/docker-ci/coverage/{docker,docker-registry}/    mapped to host volumes
/data/buildbot/{master,slave}/   main docker-ci buildbot config and database
/var/socket/{docker.sock}        host volume access to docker socket


Production deployment
=====================

::

  # Clone docker-ci repository
  git clone https://github.com/dotcloud/docker
  cd docker/hack/infrastructure/docker-ci

  export DOCKER_PROD=[PRODUCTION_SERVER_IP]

  # Create data host volume. (only once)
  docker -H $DOCKER_PROD run -v /home:/data ubuntu:12.04 \
    mkdir -p /data/docker-ci/coverage/docker
  docker -H $DOCKER_PROD run -v /home:/data ubuntu:12.04 \
    mkdir -p /data/docker-ci/coverage/docker-registry
  docker -H $DOCKER_PROD run -v /home:/data ubuntu:12.04 \
    chown -R 1000.1000 /data/docker-ci

  # dcr deployment. Define credentials and special environment dcr variables
  # ( retrieved at /hack/infrastructure/docker-ci/dcr/prod/docker-ci.yml )
  export WEB_USER=[DOCKER-CI-WEBSITE-USERNAME]
  export WEB_IRC_PWD=[DOCKER-CI-WEBSITE-PASSWORD]
  export BUILDBOT_PWD=[BUILDSLAVE_PASSWORD]
  export AWS_ACCESS_KEY=[DOCKER_RELEASE_S3_ACCESS]
  export AWS_SECRET_KEY=[DOCKER_RELEASE_S3_SECRET]
  export GPG_PASSPHRASE=[DOCKER_RELEASE_PASSPHRASE]
  export BACKUP_AWS_ID=[S3_BUCKET_CREDENTIAL_ACCESS]
  export BACKUP_AWS_SECRET=[S3_BUCKET_CREDENTIAL_SECRET]
  export SMTP_USER=[MAILGUN_SMTP_USERNAME]
  export SMTP_PWD=[MAILGUN_SMTP_PASSWORD]
  export EMAIL_RCP=[EMAIL_FOR_BUILD_ERRORS]

  # Build docker-ci and testbuilder docker images
  docker -H $DOCKER_PROD build -rm -t docker-ci/docker-ci .
  (cd testbuilder; docker -H $DOCKER_PROD build -rm -t docker-ci/testbuilder .)

  # Run docker-ci container ( assuming no previous container running )
  (cd dcr/prod; dcr docker-ci.yml start)
  (cd dcr/prod; dcr docker-ci.yml register docker-ci.docker.io)
