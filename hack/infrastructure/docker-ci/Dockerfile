# VERSION:        0.22
# DOCKER-VERSION  0.6.3
# AUTHOR:         Daniel Mizyrycki <daniel@dotcloud.com>
# DESCRIPTION:    Deploy docker-ci on Digital Ocean
# COMMENTS:
#     CONFIG_JSON is an environment variable json string loaded as:
#
# export CONFIG_JSON='
#     { "DROPLET_NAME":        "docker-ci",
#       "DO_CLIENT_ID":        "Digital_Ocean_client_id",
#       "DO_API_KEY":          "Digital_Ocean_api_key",
#       "DOCKER_KEY_ID":       "Digital_Ocean_ssh_key_id",
#       "DOCKER_CI_KEY_PATH":  "docker-ci_private_key_path",
#       "DOCKER_CI_PUB":       "$(cat docker-ci_ssh_public_key.pub)",
#       "DOCKER_CI_KEY":       "$(cat docker-ci_ssh_private_key.key)",
#       "BUILDBOT_PWD":        "Buildbot_server_password",
#       "IRC_PWD":             "Buildbot_IRC_password",
#       "SMTP_USER":           "SMTP_server_user",
#       "SMTP_PWD":            "SMTP_server_password",
#       "PKG_ACCESS_KEY":      "Docker_release_S3_bucket_access_key",
#       "PKG_SECRET_KEY":      "Docker_release_S3_bucket_secret_key",
#       "PKG_GPG_PASSPHRASE":  "Docker_release_gpg_passphrase",
#       "INDEX_AUTH":          "Index_encripted_user_password",
#       "REGISTRY_USER":       "Registry_test_user",
#       "REGISTRY_PWD":        "Registry_test_password",
#       "REGISTRY_BUCKET":     "Registry_S3_bucket_name",
#       "REGISTRY_ACCESS_KEY": "Registry_S3_bucket_access_key",
#       "REGISTRY_SECRET_KEY": "Registry_S3_bucket_secret_key",
#       "IRC_CHANNEL":         "Buildbot_IRC_channel",
#       "EMAIL_RCP":           "Buildbot_mailing_receipient" }'
#
#
# TO_BUILD:   docker build -t docker-ci .
# TO_DEPLOY:  docker run -e CONFIG_JSON="${CONFIG_JSON}" docker-ci

from ubuntu:12.04

run echo 'deb http://archive.ubuntu.com/ubuntu precise main universe' \
    > /etc/apt/sources.list
run apt-get update; apt-get install -y git python2.7 python-dev libevent-dev \
    python-pip ssh rsync less vim
run pip install requests fabric

# Add deployment code and set default container command
add . /docker-ci
cmd "/docker-ci/deployment.py"

