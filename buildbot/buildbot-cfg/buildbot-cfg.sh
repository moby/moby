#!/bin/bash

# Auto setup of buildbot configuration. Package installation is being done
# on buildbot.pp
# Dependencies: buildbot, buildbot-slave, supervisor

SLAVE_NAME='buildworker'
SLAVE_SOCKET='localhost:9989'
BUILDBOT_PWD='pass-docker'
USER='vagrant'
ROOT_PATH='/data/buildbot'
DOCKER_PATH='/data/docker'
BUILDBOT_CFG="$DOCKER_PATH/buildbot/buildbot-cfg"
IP=$(grep BUILDBOT_IP /data/docker/buildbot/Vagrantfile | awk -F "'" '{ print $2; }')

function run { su $USER -c "$1"; }

export PATH=/bin:sbin:/usr/bin:/usr/sbin:/usr/local/bin

# Exit if buildbot has already been installed
[ -d "$ROOT_PATH" ] && exit 0

# Setup buildbot
run "mkdir -p ${ROOT_PATH}"
cd ${ROOT_PATH}
run "buildbot create-master master"
run "cp $BUILDBOT_CFG/master.cfg master"
run "sed -i 's/localhost/$IP/' master/master.cfg"
run "buildslave create-slave slave $SLAVE_SOCKET $SLAVE_NAME $BUILDBOT_PWD"

# Allow buildbot subprocesses (docker tests) to properly run in containers,
# in particular with docker -u
run "sed -i 's/^umask = None/umask = 000/' ${ROOT_PATH}/slave/buildbot.tac"

# Setup supervisor
cp $BUILDBOT_CFG/buildbot.conf /etc/supervisor/conf.d/buildbot.conf
sed -i "s/^chmod=0700.*0700./chmod=0770\nchown=root:$USER/" /etc/supervisor/supervisord.conf
kill -HUP `pgrep -f "/usr/bin/python /usr/bin/supervisord"`

# Add git hook
cp $BUILDBOT_CFG/post-commit $DOCKER_PATH/.git/hooks
sed -i "s/localhost/$IP/" $DOCKER_PATH/.git/hooks/post-commit

