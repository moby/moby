#!/usr/bin/env python
import subprocess

from env import commit_range

files = subprocess.check_output(["git", "diff", "--diff-filter=ACMR",
                                 "--name-only", commit_range])

exit_status = 0

for filename in files.split('\n'):
    if filename.endswith('.go'):
        try:
            subprocess.check_call(['gofmt', filename])
        except subprocess.CalledProcessError:
            exit_status = 1

if exit_status == 0:
    print "All files pass gofmt."
else:
    print "Reformat the files listed above with 'gofmt -w' and try again."

exit(exit_status)
