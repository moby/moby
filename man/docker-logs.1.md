% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-logs - Fetch the logs of a container

# SYNOPSIS
**docker logs**
[**-f**|**--follow**]
[**--help**]
[**--since**[=*SINCE*]]
[**-t**|**--timestamps**]
[**--tail**[=*"all"*]]
CONTAINER

# DESCRIPTION
The **docker logs** command batch-retrieves whatever logs are present for
a container at the time of execution. This does not guarantee execution
order when combined with a docker run (i.e., your run may not have generated
any logs at the time you execute docker logs).

The **docker logs --follow** command combines commands **docker logs** and
**docker attach**. It will first return all logs from the beginning and
then continue streaming new output from the container’s stdout and stderr.

**Warning**: This command works only for the **json-file** or **journald**
logging drivers.

# OPTIONS
**--help**
  Print usage statement

**--details**=*true*|*false*
   Show extra details provided to logs

**-f**, **--follow**=*true*|*false*
   Follow log output. The default is *false*.

**--since**=""
   Show logs since timestamp

**-t**, **--timestamps**=*true*|*false*
   Show timestamps. The default is *false*.

**--tail**="*all*"
   Output the specified number of lines at the end of logs (defaults to all logs)

The `--since` option can be Unix timestamps, date formatted timestamps, or Go
duration strings (e.g. `10m`, `1h30m`) computed relative to the client machine’s
time. Supported formats for date formatted time stamps include RFC3339Nano,
RFC3339, `2006-01-02T15:04:05`, `2006-01-02T15:04:05.999999999`,
`2006-01-02Z07:00`, and `2006-01-02`. The local timezone on the client will be
used if you do not provide either a `Z` or a `+-00:00` timezone offset at the
end of the timestamp.  When providing Unix timestamps enter
seconds[.nanoseconds], where seconds is the number of seconds that have elapsed
since January 1, 1970 (midnight UTC/GMT), not counting leap  seconds (aka Unix
epoch or Unix time), and the optional .nanoseconds field is a fraction of a
second no more than nine digits long. You can combine the `--since` option with
either or both of the `--follow` or `--tail` options.

The `docker logs --details` command will add on extra attributes, such as
environment variables and labels, provided to `--log-opt` when creating the
container.

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
July 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
April 2015, updated by Ahmet Alp Balkan <ahmetalpbalkan@gmail.com>
October 2015, updated by Mike Brown <mikebrow@gmail.com>
