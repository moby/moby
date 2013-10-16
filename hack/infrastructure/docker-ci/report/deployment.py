#!/usr/bin/env python

'''Deploy docker-ci report container on Digital Ocean.
Usage:
    export CONFIG_JSON='
        { "DROPLET_NAME":        "Digital_Ocean_dropplet_name",
          "DO_CLIENT_ID":        "Digital_Ocean_client_id",
          "DO_API_KEY":          "Digital_Ocean_api_key",
          "DOCKER_KEY_ID":       "Digital_Ocean_ssh_key_id",
          "DOCKER_CI_KEY_PATH":  "docker-ci_private_key_path",
          "DOCKER_CI_PUB":       "$(cat docker-ci_ssh_public_key.pub)",
          "DOCKER_CI_ADDRESS"    "user@docker-ci_fqdn_server",
          "SMTP_USER":           "SMTP_server_user",
          "SMTP_PWD":            "SMTP_server_password",
          "EMAIL_SENDER":        "Buildbot_mailing_sender",
          "EMAIL_RCP":           "Buildbot_mailing_receipient" }'
    python deployment.py
'''

import re, json, requests, base64
from fabric import api
from fabric.api import cd, run, put, sudo
from os import environ as env
from time import sleep
from datetime import datetime

# Populate environment variables
CONFIG = json.loads(env['CONFIG_JSON'])
for key in CONFIG:
    env[key] = CONFIG[key]

# Load DOCKER_CI_KEY
env['DOCKER_CI_KEY'] = open(env['DOCKER_CI_KEY_PATH']).read()

DROPLET_NAME = env.get('DROPLET_NAME','report')
TIMEOUT = 120            # Seconds before timeout droplet creation
IMAGE_ID = 894856        # Docker on Ubuntu 13.04
REGION_ID = 4            # New York 2
SIZE_ID = 66             # memory 512MB
DO_IMAGE_USER = 'root'   # Image user on Digital Ocean
API_URL = 'https://api.digitalocean.com/'


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

api.env.host_string = ip
api.env.user = DO_IMAGE_USER
api.env.key_filename = env['DOCKER_CI_KEY_PATH']

# Correct timezone
sudo('echo "America/Los_Angeles" >/etc/timezone')
sudo('dpkg-reconfigure --frontend noninteractive tzdata')

# Load JSON_CONFIG environment for Dockerfile
CONFIG_JSON= base64.b64encode(
    '{{"DOCKER_CI_PUB":     "{DOCKER_CI_PUB}",'
    '  "DOCKER_CI_KEY":     "{DOCKER_CI_KEY}",'
    '  "DOCKER_CI_ADDRESS": "{DOCKER_CI_ADDRESS}",'
    '  "SMTP_USER":         "{SMTP_USER}",'
    '  "SMTP_PWD":          "{SMTP_PWD}",'
    '  "EMAIL_SENDER":      "{EMAIL_SENDER}",'
    '  "EMAIL_RCP":         "{EMAIL_RCP}"}}'.format(**env))

run('mkdir -p /data/report')
put('./', '/data/report')
with cd('/data/report'):
    run('chmod 700 report.py')
    run('echo "{}" > credentials.json'.format(CONFIG_JSON))
    run('docker build -t report .')
    run('rm credentials.json')
    run("echo -e '30 09 * * * /usr/bin/docker run report\n' |"
        " /usr/bin/crontab -")
