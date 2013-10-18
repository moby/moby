#!/bin/bash

# Setup of buildbot configuration. Package installation is being done by
# Vagrantfile
# Dependencies: buildbot, buildbot-slave, supervisor

USER=$1
CFG_PATH=$2
DOCKER_PATH=$3
BUILDBOT_PWD=$4
IRC_PWD=$5
IRC_CHANNEL=$6
SMTP_USER=$7
SMTP_PWD=$8
EMAIL_RCP=$9
REGISTRY_USER=${10}
REGISTRY_PWD=${11}
REGISTRY_BUCKET=${12}
REGISTRY_ACCESS_KEY=${13}
REGISTRY_SECRET_KEY=${14}
BUILDBOT_PATH="/data/buildbot"
SLAVE_NAME="buildworker"
SLAVE_SOCKET="localhost:9989"

export PATH="/bin:sbin:/usr/bin:/usr/sbin:/usr/local/bin"

function run { su $USER -c "$1"; }

# Exit if buildbot has already been installed
[ -d "$BUILDBOT_PATH" ] && exit 0

# Setup buildbot
run "mkdir -p $BUILDBOT_PATH"
cd $BUILDBOT_PATH
run "buildbot create-master master"
run "cp $CFG_PATH/master.cfg master"
run "sed -i -E 's#(BUILDBOT_PWD = ).+#\1\"$BUILDBOT_PWD\"#' master/master.cfg"
run "sed -i -E 's#(IRC_PWD = ).+#\1\"$IRC_PWD\"#' master/master.cfg"
run "sed -i -E 's#(IRC_CHANNEL = ).+#\1\"$IRC_CHANNEL\"#' master/master.cfg"
run "sed -i -E 's#(SMTP_USER = ).+#\1\"$SMTP_USER\"#' master/master.cfg"
run "sed -i -E 's#(SMTP_PWD = ).+#\1\"$SMTP_PWD\"#' master/master.cfg"
run "sed -i -E 's#(EMAIL_RCP = ).+#\1\"$EMAIL_RCP\"#' master/master.cfg"
run "buildslave create-slave slave $SLAVE_SOCKET $SLAVE_NAME $BUILDBOT_PWD"
run "echo 'export DOCKER_CREDS=\"$REGISTRY_USER:$REGISTRY_PWD\"' > $BUILDBOT_PATH/master/credentials.cfg"
run "echo 'export S3_BUCKET=\"$REGISTRY_BUCKET\"' >> $BUILDBOT_PATH/master/credentials.cfg"
run "echo 'export S3_ACCESS_KEY=\"$REGISTRY_ACCESS_KEY\"' >> $BUILDBOT_PATH/master/credentials.cfg"
run "echo 'export S3_SECRET_KEY=\"$REGISTRY_SECRET_KEY\"' >> $BUILDBOT_PATH/master/credentials.cfg"

# Patch github webstatus to capture pull requests
cp $CFG_PATH/github.py /usr/local/lib/python2.7/dist-packages/buildbot/status/web/hooks

# Allow buildbot subprocesses (docker tests) to properly run in containers,
# in particular with docker -u
run "sed -i 's/^umask = None/umask = 000/' slave/buildbot.tac"

# Setup supervisor
cp $CFG_PATH/buildbot.conf /etc/supervisor/conf.d/buildbot.conf
sed -i -E "s/^chmod=0700.+/chmod=0770\nchown=root:$USER/" /etc/supervisor/supervisord.conf
kill -HUP $(pgrep -f "/usr/bin/python /usr/bin/supervisord")
