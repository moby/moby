#!/usr/bin/env python
import subprocess

from env import commit_range

files = subprocess.check_output([
	'git', 'diff', '--diff-filter=ACMR',
	'--name-only', '...'.join(commit_range), '--',
])

exit_status = 0

for filename in files.split('\n'):
	if filename.startswith('vendor/'):
		continue # we can't be changing our upstream vendors for gofmt, so don't even check them
	
	if filename.endswith('.go'):
		try:
			out = subprocess.check_output(['gofmt', '-s', '-l', filename])
			if out != '':
				print out,
				exit_status = 1
		except subprocess.CalledProcessError:
			exit_status = 1

if exit_status != 0:
	print 'Reformat the files listed above with "gofmt -s -w" and try again.'
	exit(exit_status)

print 'All files pass gofmt.'
exit(0)
