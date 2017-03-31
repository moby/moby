#!/usr/bin/env python

'Parse main CHANGELOG.md from stdin outputing on stdout the debian changelog'

import sys,re, datetime

on_block=False
for line in sys.stdin.readlines():
    line = line.strip()
    if line.startswith('# ') or len(line) == 0:
        continue
    if line.startswith('## '):
        if on_block:
            print '\n -- dotCloud <ops@dotcloud.com>  {0}\n'.format(date)
        version, date = line[3:].split()
        date = datetime.datetime.strptime(date, '(%Y-%m-%d)').strftime(
            '%a, %d %b %Y 00:00:00 -0700')
        on_block = True
        print 'lxc-docker ({0}-1) precise; urgency=low'.format(version)
        continue
    if on_block:
        print '  ' + line
print '\n -- dotCloud <ops@dotcloud.com>  {0}'.format(date)
