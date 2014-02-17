#!/usr/bin/env python

import os,sys,json
from datetime import datetime
from filecmp import cmp
from subprocess import check_call
from boto.s3.key import Key
from boto.s3.connection import S3Connection

def ENV(x):
    '''Promote an environment variable for global use returning its value'''
    retval = os.environ.get(x, '')
    globals()[x] = retval
    return retval

ROOT_PATH = '/data/backup/docker-ci'
TODAY = str(datetime.today())[:10]
BACKUP_FILE = '{}/docker-ci_{}.tgz'.format(ROOT_PATH, TODAY)
BACKUP_LINK = '{}/docker-ci.tgz'.format(ROOT_PATH)
ENV('BACKUP_BUCKET')
ENV('BACKUP_AWS_ID')
ENV('BACKUP_AWS_SECRET')

'''Create full master buildbot backup, avoiding duplicates'''
# Ensure backup path exist
if not os.path.exists(ROOT_PATH):
    os.makedirs(ROOT_PATH)
# Make actual backups
check_call('/bin/tar czf {} -C /data --exclude=backup --exclude=buildbot/slave'
    ' . 1>/dev/null 2>&1'.format(BACKUP_FILE),shell=True)
# remove previous dump if it is the same as the latest
if (os.path.exists(BACKUP_LINK) and cmp(BACKUP_FILE, BACKUP_LINK) and
 os.path._resolve_link(BACKUP_LINK) != BACKUP_FILE):
    os.unlink(os.path._resolve_link(BACKUP_LINK))
# Recreate backup link pointing to latest backup
try:
    os.unlink(BACKUP_LINK)
except:
    pass
os.symlink(BACKUP_FILE, BACKUP_LINK)

# Make backup on S3
bucket = S3Connection(BACKUP_AWS_ID,BACKUP_AWS_SECRET).get_bucket(BACKUP_BUCKET)
k = Key(bucket)
k.key = BACKUP_FILE
k.set_contents_from_filename(BACKUP_FILE)
bucket.copy_key(os.path.basename(BACKUP_LINK),BACKUP_BUCKET,BACKUP_FILE[1:])
