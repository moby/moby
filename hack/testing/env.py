import os
import subprocess

if "TRAVIS" not in os.environ:
    print "TRAVIS is not defined; this should run in TRAVIS. Sorry."
    exit(3)

if os.environ["TRAVIS_PULL_REQUEST"] != "false":
    commit_range = "{TRAVIS_BRANCH}..FETCH_HEAD".format(**os.environ)
else:
    try:
        subprocess.check_call(["git", "log", "-1", "--format=format:",
                               os.environ["TRAVIS_COMMIT_RANGE"]])
        commit_range = os.environ["TRAVIS_COMMIT_RANGE"]
    except subprocess.CalledProcessError:
        print "TRAVIS_COMMIT_RANGE is invalid."
        print "This seems to be a force push."
        print "Only checking the last 10 commits."
        commit_range = "HEAD^10..HEAD"
