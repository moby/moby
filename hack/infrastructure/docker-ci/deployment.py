#!/usr/bin/env python

import os, sys, re, json, base64
from boto.ec2.connection import EC2Connection
from subprocess import call
from fabric import api
from fabric.api import cd, run, put, sudo
from os import environ as env
from time import sleep

# Remove SSH private key as it needs more processing
CONFIG = json.loads(re.sub(r'("DOCKER_CI_KEY".+?"(.+?)",)','',
    env['CONFIG_JSON'], flags=re.DOTALL))

# Populate environment variables
for key in CONFIG:
    env[key] = CONFIG[key]

# Load SSH private key
env['DOCKER_CI_KEY'] = re.sub('^.+"DOCKER_CI_KEY".+?"(.+?)".+','\\1',
    env['CONFIG_JSON'],flags=re.DOTALL)


AWS_TAG = env.get('AWS_TAG','docker-ci')
AWS_KEY_NAME = 'dotcloud-dev'       # Same as CONFIG_JSON['DOCKER_CI_PUB']
AWS_AMI = 'ami-d582d6bc'            # Ubuntu 13.04
AWS_REGION = 'us-east-1'
AWS_TYPE = 'm1.small'
AWS_SEC_GROUPS = 'gateway'
AWS_IMAGE_USER = 'ubuntu'
DOCKER_PATH = '/go/src/github.com/dotcloud/docker'
DOCKER_CI_PATH = '/docker-ci'
CFG_PATH = '{}/buildbot'.format(DOCKER_CI_PATH)


class AWS_EC2:
    '''Amazon EC2'''
    def __init__(self, access_key, secret_key):
        '''Set default API parameters'''
        self.handler = EC2Connection(access_key, secret_key)
    def create_instance(self, tag, instance_type):
        reservation = self.handler.run_instances(**instance_type)
        instance = reservation.instances[0]
        sleep(10)
        while instance.state != 'running':
            sleep(5)
            instance.update()
            print "Instance state: %s" % (instance.state)
        instance.add_tag("Name",tag)
        print "instance %s done!" % (instance.id)
        return instance.ip_address
    def get_instances(self):
        return self.handler.get_all_instances()
    def get_tags(self):
        return dict([(i.instances[0].id, i.instances[0].tags['Name'])
            for i in self.handler.get_all_instances() if i.instances[0].tags])
    def del_instance(self, instance_id):
        self.handler.terminate_instances(instance_ids=[instance_id])


def json_fmt(data):
    '''Format json output'''
    return json.dumps(data, sort_keys = True, indent = 2)


# Create EC2 API handler
ec2 = AWS_EC2(env['AWS_ACCESS_KEY'], env['AWS_SECRET_KEY'])

# Stop processing if AWS_TAG exists on EC2
if AWS_TAG in ec2.get_tags().values():
    print ('Instance: {} already deployed. Not further processing.'
        .format(AWS_TAG))
    exit(1)

ip = ec2.create_instance(AWS_TAG, {'image_id':AWS_AMI, 'instance_type':AWS_TYPE,
    'security_groups':[AWS_SEC_GROUPS], 'key_name':AWS_KEY_NAME})

# Wait 30 seconds for the machine to boot
sleep(30)

# Create docker-ci ssh private key so docker-ci docker container can communicate
# with its EC2 instance
os.makedirs('/root/.ssh')
open('/root/.ssh/id_rsa','w').write(env['DOCKER_CI_KEY'])
os.chmod('/root/.ssh/id_rsa',0600)
open('/root/.ssh/config','w').write('StrictHostKeyChecking no\n')

api.env.host_string = ip
api.env.user = AWS_IMAGE_USER
api.env.key_filename = '/root/.ssh/id_rsa'

# Correct timezone
sudo('echo "America/Los_Angeles" >/etc/timezone')
sudo('dpkg-reconfigure --frontend noninteractive tzdata')

# Load public docker-ci key
sudo("echo '{}' >> /root/.ssh/authorized_keys".format(env['DOCKER_CI_PUB']))

# Create docker nightly release credentials file
credentials = {
    'AWS_ACCESS_KEY': env['PKG_ACCESS_KEY'],
    'AWS_SECRET_KEY': env['PKG_SECRET_KEY'],
    'GPG_PASSPHRASE': env['PKG_GPG_PASSPHRASE'],
    'INDEX_AUTH': env['INDEX_AUTH']}
open(DOCKER_CI_PATH + '/nightlyrelease/release_credentials.json', 'w').write(
    base64.b64encode(json.dumps(credentials)))

# Transfer docker
sudo('mkdir -p ' + DOCKER_CI_PATH)
sudo('chown {}.{} {}'.format(AWS_IMAGE_USER, AWS_IMAGE_USER, DOCKER_CI_PATH))
call('/usr/bin/rsync -aH {} {}@{}:{}'.format(DOCKER_CI_PATH, AWS_IMAGE_USER, ip,
    os.path.dirname(DOCKER_CI_PATH)), shell=True)

# Install Docker and Buildbot dependencies
sudo('addgroup docker')
sudo('usermod -a -G docker ubuntu')
sudo('mkdir /mnt/docker; ln -s /mnt/docker /var/lib/docker')
sudo('wget -q -O - https://get.docker.io/gpg | apt-key add -')
sudo('echo deb https://get.docker.io/ubuntu docker main >'
    ' /etc/apt/sources.list.d/docker.list')
sudo('echo -e "deb http://archive.ubuntu.com/ubuntu raring main universe\n'
    'deb http://us.archive.ubuntu.com/ubuntu/ raring-security main universe\n"'
    ' > /etc/apt/sources.list; apt-get update')
sudo('DEBIAN_FRONTEND=noninteractive apt-get install -q -y wget python-dev'
    ' python-pip supervisor git mercurial linux-image-extra-$(uname -r)'
    ' aufs-tools make libfontconfig libevent-dev')
sudo('wget -O - https://go.googlecode.com/files/go1.1.2.linux-amd64.tar.gz | '
    'tar -v -C /usr/local -xz; ln -s /usr/local/go/bin/go /usr/bin/go')
sudo('GOPATH=/go go get -d github.com/dotcloud/docker')
sudo('pip install -r {}/requirements.txt'.format(CFG_PATH))

# Install docker and testing dependencies
sudo('apt-get install -y -q lxc-docker')
sudo('curl -s https://phantomjs.googlecode.com/files/'
    'phantomjs-1.9.1-linux-x86_64.tar.bz2 | tar jx -C /usr/bin'
    ' --strip-components=2 phantomjs-1.9.1-linux-x86_64/bin/phantomjs')

# Preventively reboot docker-ci daily
sudo('ln -s /sbin/reboot /etc/cron.daily')

# Preventively reboot docker-ci daily
sudo('ln -s /sbin/reboot /etc/cron.daily')

# Build docker-ci containers
sudo('cd {}; docker build -t docker .'.format(DOCKER_PATH))
sudo('cd {}/nightlyrelease; docker build -t dockerbuilder .'.format(
    DOCKER_CI_PATH))

# Download docker-ci testing container
sudo('docker pull mzdaniel/test_docker')

# Setup buildbot
sudo('mkdir /data')
sudo('{0}/setup.sh root {0} {1} {2} {3} {4} {5} {6} {7} {8} {9} {10}'
    ' {11} {12}'.format(CFG_PATH, DOCKER_PATH, env['BUILDBOT_PWD'],
    env['IRC_PWD'], env['IRC_CHANNEL'], env['SMTP_USER'],
    env['SMTP_PWD'], env['EMAIL_RCP'], env['REGISTRY_USER'],
    env['REGISTRY_PWD'], env['REGISTRY_BUCKET'], env['REGISTRY_ACCESS_KEY'],
    env['REGISTRY_SECRET_KEY']))
