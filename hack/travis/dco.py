#!/usr/bin/env python
import re
import subprocess
import yaml

from env import commit_range

commit_format = '-%n hash: "%h"%n author: %aN <%aE>%n message: |%n%w(0,2,2).%B'

gitlog = subprocess.check_output([
	'git', 'log', '--reverse',
	'--format=format:'+commit_format,
	'..'.join(commit_range), '--',
])

commits = yaml.load(gitlog)
if not commits:
	exit(0) # what?  how can we have no commits?

DCO = 'Docker-DCO-1.1-Signed-off-by:'

p = re.compile(r'^{0} ([^<]+) <([^<>@]+@[^<>]+)> \(github: (\S+)\)$'.format(re.escape(DCO)), re.MULTILINE|re.UNICODE)

failed_commits = 0

for commit in commits:
	commit['message'] = commit['message'][1:]
	# trim off our '.' that exists just to prevent fun YAML parsing issues
	# see https://github.com/dotcloud/docker/pull/3836#issuecomment-33723094
	# and https://travis-ci.org/dotcloud/docker/builds/17926783
	
	commit['stat'] = subprocess.check_output([
		'git', 'log', '--format=format:', '--max-count=1',
		'--name-status', commit['hash'], '--',
	])
	if commit['stat'] == '':
		print 'Commit {0} has no actual changed content, skipping.'.format(commit['hash'])
		continue
	
	m = p.search(commit['message'])
	if not m:
		print 'Commit {1} does not have a properly formatted "{0}" marker.'.format(DCO, commit['hash'])
		failed_commits += 1
		continue # print ALL the commits that don't have a proper DCO
	
	(name, email, github) = m.groups()
	
	# TODO verify that "github" is the person who actually made this commit via the GitHub API

if failed_commits > 0:
	exit(failed_commits)

print 'All commits have a valid "{0}" marker.'.format(DCO)
exit(0)
