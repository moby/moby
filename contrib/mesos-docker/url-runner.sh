#!/bin/bash
# This script allows running an arbitrary url via chronos and ensuring we don't always have to download it if it exists already.
# Note that this means that you should version your jobs. You can use this on top of the mesos-command executor.

set -o nounset	# exit if trying to use an uninitialized var
set -o errexit	# exit if any program fails
set -o pipefail # exit if any program in a pipeline fails, also
set -x          # debug mode

# the main dispatcher for running async jobs on the emr cluster
job_url="$1" ; shift
job_args="$@"

# get some environment variables
source /etc/environment
export HOME="/tmp" 			#required for things like RDS CLI

# go to the directory where we're going to store the jobs
job_dir="/mnt/mesos_url_jobs"
mkdir -p $job_dir

# get the url
if [[ $job_url == "http://"* || $job_url == "https://"* ]]
then
        job_clean=${job_url%%\?*}
	job_path="${job_dir}/`basename $job_clean`"
	if [[ ! -e "${job_path}" ]]
	then
		mkdir -p `dirname ${job_path}`
		wget --no-verbose --no-check-certificate -O "${job_path}" "${job_url}"
	fi
fi

chmod u+x $job_path
$job_path $job_args
