% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-logs - Fetch the logs of a container

# SYNOPSIS
**docker logs** **--follow**[=*false*] CONTAINER

# DESCRIPTION
The **docker logs** command batch-retrieves whatever logs are present for
a container at the time of execution. This does not guarantee execution
order when combined with a docker run (i.e. your run may not have generated
any logs at the time you execute docker logs).

The **docker logs --follow** command combines commands **docker logs** and
**docker attach**. It will first return all logs from the beginning and
then continue streaming new output from the container’s stdout and stderr.

# OPTIONS
**-f, --follow**=*true*|*false*
   When *true*, follow log output. The default is false.

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.
