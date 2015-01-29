% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-events - Get real time events from the server

# SYNOPSIS
**docker events**
[**--help**]
[**-f**|**--filter**[=*[]*]]
[**--since**[=*SINCE*]]
[**--until**[=*UNTIL*]]


# DESCRIPTION
Get event information from the Docker daemon. Information can include historical
information and real-time information.

Docker containers will report the following events:

    create, destroy, die, export, kill, pause, restart, start, stop, unpause

and Docker images will report:

    untag, delete

# OPTIONS
**--help**
  Print usage statement

**-f**, **--filter**=[]
   Provide filter values (i.e., 'event=stop')

**--since**=""
   Show all events created since timestamp

**--until**=""
   Stream events until this timestamp

# EXAMPLES

## Listening for Docker events

After running docker events a container 786d698004576 is started and stopped
(The container name has been shortened in the output below):

    # docker events
    2015-01-28T20:21:31.000000000-08:00 59211849bc10: (from whenry/testimage:latest) start
    2015-01-28T20:21:31.000000000-08:00 59211849bc10: (from whenry/testimage:latest) die
    2015-01-28T20:21:32.000000000-08:00 59211849bc10: (from whenry/testimage:latest) stop

## Listening for events since a given date
Again the output container IDs have been shortened for the purposes of this document:

    # docker events --since '2015-01-28'
    2015-01-28T20:25:38.000000000-08:00 c21f6c22ba27: (from whenry/testimage:latest) create
    2015-01-28T20:25:38.000000000-08:00 c21f6c22ba27: (from whenry/testimage:latest) start
    2015-01-28T20:25:39.000000000-08:00 c21f6c22ba27: (from whenry/testimage:latest) create
    2015-01-28T20:25:39.000000000-08:00 c21f6c22ba27: (from whenry/testimage:latest) start
    2015-01-28T20:25:40.000000000-08:00 c21f6c22ba27: (from whenry/testimage:latest) die
    2015-01-28T20:25:42.000000000-08:00 c21f6c22ba27: (from whenry/testimage:latest) stop
    2015-01-28T20:25:45.000000000-08:00 c21f6c22ba27: (from whenry/testimage:latest) start
    2015-01-28T20:25:45.000000000-08:00 c21f6c22ba27: (from whenry/testimage:latest) die
    2015-01-28T20:25:46.000000000-08:00 c21f6c22ba27: (from whenry/testimage:latest) stop

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
