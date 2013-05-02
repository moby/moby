#!/bin/bash

# Setup of buildbot configuration. Package installation is being done by
# Vagrantfile
# Dependencies: buildbot, buildbot-slave, supervisor

USER=$1
GOPATH=$2
DOCKER_PATH=$3
CFG_PATH=$4
BUILDBOT_PATH=$5
SLAVE_NAME="buildworker"
SLAVE_SOCKET="localhost:9989"
BUILDBOT_PWD="pass-docker"
IP=$(sed -nE 's/VM_IP = "(.+)"/\1/p' ${DOCKER_PATH}/hack/Vagrantfile)
export PATH="/bin:sbin:/usr/bin:/usr/sbin:/usr/local/bin"

function run { su $USER -c "$1"; }

# Exit if buildbot has already been installed
[ -d "$BUILDBOT_PATH" ] && exit 0

# Setup buildbot
run "mkdir -p $BUILDBOT_PATH"
cd $BUILDBOT_PATH
run "buildbot create-master master"
run "cp $CFG_PATH/master.cfg master"
run "sed -i 's/localhost/$IP/' master/master.cfg"
run "sed -i -E 's#(GOPATH = ).+#\1\"$GOPATH\"#' master/master.cfg"
run "sed -i -E 's#(DOCKER_PATH = ).+#\1\"$DOCKER_PATH\"#' master/master.cfg"
run "buildslave create-slave slave $SLAVE_SOCKET $SLAVE_NAME $BUILDBOT_PWD"

# Allow buildbot subprocesses (docker tests) to properly run in containers,
# in particular with docker -u
run "sed -i 's/^umask = None/umask = 000/' slave/buildbot.tac"

# Setup supervisor
cp $CFG_PATH/buildbot.conf /etc/supervisor/conf.d/buildbot.conf
sed -i -E "s/^chmod=0700.+/chmod=0770\nchown=root:$USER/" /etc/supervisor/supervisord.conf
kill -HUP $(pgrep -f "/usr/bin/python /usr/bin/supervisord")

# Add git hook
cp $CFG_PATH/post-commit $DOCKER_PATH/.git/hooks
sed -i "s/localhost/$IP/" $DOCKER_PATH/.git/hooks/post-commit

