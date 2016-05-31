# Splunk Logging Driver for Docker - Functional Tests # 

**Quick summary:** These are a small set of functional tests to evaluate the function and performance of the Splunk Logging Driver for Docker. This README will guide a new developer through the steps required to run functional tests locally.

### Instructions: ###

**Dependencies:**

1.) Docker installed

2.) This repo cloned locally

**To Run:**

1.) Checkout branch to run the log driver tests against ("git checkout <branchname>; git pull")

2.) In the root directory of this repo, simply run "make test-log-drivers". This will build the latest docker from source and run all the log driver tests, including the one for Splunk.
