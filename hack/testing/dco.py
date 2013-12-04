#!/usr/bin/env python
import subprocess
import yaml

from env import commit_range

commit_format = "-%n hash: %h%n author: %an <%ae>%n message: |%n%w(1000,2,2)%B"

gitlog = subprocess.check_output(["git", "log", "--reverse",
                                  "--format=format:"+commit_format,
                                  commit_range])

commits = yaml.load(gitlog)

prefix = "Docker-DCO-1.0-Signed-off-by: "

for commit in commits:
    if prefix not in commit["message"]:
        print ("Commit {0} does not have {1!r} marker!"
               .format(commit["hash"], prefix))
        exit(2)
    signoff = "\n" + prefix + commit["author"] + "\n"
    if signoff not in commit["message"]:
        print ("Commit {0} does have a {1!r} marker, but it looks like "
               "it does not match the author ({2})!"
               .format(commit["hash"], prefix, commit["author"]))
        exit(2)

print "All commits have a valid {0!r} marker.".format(prefix)
exit(0)
