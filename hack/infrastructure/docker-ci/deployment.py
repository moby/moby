#!/usr/bin/env python

import os, sys, re, json, requests, base64
from subprocess import call
from fabric import api
from fabric.api import cd, run, put, sudo
from os import environ as env
from datetime import datetime
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

DROPLET_NAME = env.get('DROPLET_NAME','docker-ci')
TIMEOUT = 120            # Seconds before timeout droplet creation
IMAGE_ID = 1004145       # Docker on Ubuntu 13.04
REGION_ID = 4            # New York 2
SIZE_ID = 62             # memory 2GB
DO_IMAGE_USER = 'root'   # Image user on Digital Ocean
API_URL = 'https://api.digitalocean.com/'
DOCKER_PATH = '/go/src/github.com/dotcloud/docker'
DOCKER_CI_PATH = '/docker-ci'
CFG_PATH = '{}/buildbot'.format(DOCKER_CI_PATH)


class digital_ocean():

    def __init__(self, key, client):
        '''Set default API parameters'''
        self.key = key
        self.client = client
        self.api_url = API_URL

    def api(self, cmd_path, api_arg={}):
        '''Make api call'''
        api_arg.update({'api_key':self.key, 'client_id':self.client})
        resp = requests.get(self.api_url + cmd_path, params=api_arg).text
        resp = json.loads(resp)
        if resp['status'] != 'OK':
            raise Exception(resp['error_message'])
        return resp

    def droplet_data(self, name):
        '''Get droplet data'''
        data = self.api('droplets')
        data = [droplet for droplet in data['droplets']
            if droplet['name'] == name]
        return data[0] if data else {}


def json_fmt(data):
    '''Format json output'''
    return json.dumps(data, sort_keys = True, indent = 2)


do = digital_ocean(env['DO_API_KEY'], env['DO_CLIENT_ID'])

# Get DROPLET_NAME data
data = do.droplet_data(DROPLET_NAME)

# Stop processing if DROPLET_NAME exists on Digital Ocean
if data:
    print ('Droplet: {} already deployed. Not further processing.'
        .format(DROPLET_NAME))
    exit(1)

# Create droplet
do.api('droplets/new', {'name':DROPLET_NAME, 'region_id':REGION_ID,
    'image_id':IMAGE_ID, 'size_id':SIZE_ID,
    'ssh_key_ids':[env['DOCKER_KEY_ID']]})

# Wait for droplet to be created.
start_time = datetime.now()
while (data.get('status','') != 'active' and (
 datetime.now()-start_time).seconds < TIMEOUT):
    data = do.droplet_data(DROPLET_NAME)
    print data['status']
    sleep(3)

# Wait for the machine to boot
sleep(15)

# Get droplet IP
ip = str(data['ip_address'])
print 'droplet: {}    ip: {}'.format(DROPLET_NAME, ip)

# Create docker-ci ssh private key so docker-ci docker container can communicate
# with its EC2 instance
os.makedirs('/root/.ssh')
open('/root/.ssh/id_rsa','w').write(env['DOCKER_CI_KEY'])
os.chmod('/root/.ssh/id_rsa',0600)
open('/root/.ssh/config','w').write('StrictHostKeyChecking no\n')

api.env.host_string = ip
api.env.user = DO_IMAGE_USER
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
    'GPG_PASSPHRASE': env['PKG_GPG_PASSPHRASE']}
open(DOCKER_CI_PATH + '/nightlyrelease/release_credentials.json', 'w').write(
    base64.b64encode(json.dumps(credentials)))

# Transfer docker
sudo('mkdir -p ' + DOCKER_CI_PATH)
sudo('chown {}.{} {}'.format(DO_IMAGE_USER, DO_IMAGE_USER, DOCKER_CI_PATH))
call('/usr/bin/rsync -aH {} {}@{}:{}'.format(DOCKER_CI_PATH, DO_IMAGE_USER, ip,
    os.path.dirname(DOCKER_CI_PATH)), shell=True)

# Install Docker and Buildbot dependencies
sudo('mkdir /mnt/docker; ln -s /mnt/docker /var/lib/docker')
sudo('wget -q -O - https://get.docker.io/gpg | apt-key add -')
sudo('echo deb https://get.docker.io/ubuntu docker main >'
    ' /etc/apt/sources.list.d/docker.list')
sudo('echo -e "deb http://archive.ubuntu.com/ubuntu raring main universe\n'
    'deb http://us.archive.ubuntu.com/ubuntu/ raring-security main universe\n"'
    ' > /etc/apt/sources.list; apt-get update')
sudo('DEBIAN_FRONTEND=noninteractive apt-get install -q -y wget python-dev'
    ' python-pip supervisor git mercurial linux-image-extra-$(uname -r)'
    ' aufs-tools make libfontconfig libevent-dev libsqlite3-dev libssl-dev')
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

# Build docker-ci containers
sudo('cd {}; docker build -t docker .'.format(DOCKER_PATH))
sudo('cd {}; docker build -t docker-ci .'.format(DOCKER_CI_PATH))
sudo('cd {}/nightlyrelease; docker build -t dockerbuilder .'.format(
    DOCKER_CI_PATH))
sudo('cd {}/registry-coverage; docker build -t registry_coverage .'.format(
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
