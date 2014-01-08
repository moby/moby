import os
import subprocess

if 'TRAVIS' not in os.environ:
	print 'TRAVIS is not defined; this should run in TRAVIS. Sorry.'
	exit(127)

if os.environ['TRAVIS_PULL_REQUEST'] != 'false':
	commit_range = [os.environ['TRAVIS_BRANCH'], 'FETCH_HEAD']
else:
	try:
		subprocess.check_call([
			'git', 'log', '-1', '--format=format:',
			os.environ['TRAVIS_COMMIT_RANGE'], '--',
		])
		commit_range = os.environ['TRAVIS_COMMIT_RANGE'].split('...')
		if len(commit_range) == 1: # if it didn't split, it must have been separated by '..' instead
			commit_range = commit_range[0].split('..')
	except subprocess.CalledProcessError:
		print 'TRAVIS_COMMIT_RANGE is invalid. This seems to be a force push. We will just assume it must be against upstream master and compare all commits in between.'
		commit_range = ['upstream/master', 'HEAD']
