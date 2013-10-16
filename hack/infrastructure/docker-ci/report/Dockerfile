# VERSION:        0.22
# DOCKER-VERSION  0.6.3
# AUTHOR:         Daniel Mizyrycki <daniel@dotcloud.com>
# DESCRIPTION:    Generate docker-ci daily report
# COMMENTS:       The build process is initiated by deployment.py
                  Report configuration is passed through ./credentials.json at
#                 deployment time.
# TO_BUILD:       docker build -t report .
# TO_DEPLOY:      docker run report

from ubuntu:12.04
maintainer Daniel Mizyrycki <daniel@dotcloud.com>

env PYTHONPATH /report


# Add report dependencies
run echo 'deb http://archive.ubuntu.com/ubuntu precise main universe' > \
    /etc/apt/sources.list
run apt-get update; apt-get install -y python2.7 python-pip ssh rsync

# Set San Francisco timezone
run echo "America/Los_Angeles" >/etc/timezone
run dpkg-reconfigure --frontend noninteractive tzdata

# Add report code and set default container command
add . /report
cmd "/report/report.py"
