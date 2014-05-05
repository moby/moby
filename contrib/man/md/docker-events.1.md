% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-events - Get real time events from the server

**docker events** **--since**=""|*epoch-time*

# DESCRIPTION
Get event information from the Docker daemon. Information can include historical
information and real-time information.

# OPTIONS
**--since**=""
Show previously created events and then stream. This can be in either
seconds since epoch, or date string.

# EXAMPLES

## Listening for Docker events

After running docker events a container 786d698004576 is started and stopped
(The container name has been shortened in the ouput below):

    # docker events
    [2014-04-12 18:23:04 -0400 EDT] 786d69800457: (from whenry/testimage:latest) start
    [2014-04-12 18:23:13 -0400 EDT] 786d69800457: (from whenry/testimage:latest) die
    [2014-04-12 18:23:13 -0400 EDT] 786d69800457: (from whenry/testimage:latest) stop

## Listening for events since a given date
Again the output container IDs have been shortened for the purposes of this document:

    # docker events --since '2014-04-12'
    [2014-04-12 18:11:28 -0400 EDT] c655dbf640dc: (from whenry/testimage:latest) create
    [2014-04-12 18:11:28 -0400 EDT] c655dbf640dc: (from whenry/testimage:latest) start
    [2014-04-12 18:14:13 -0400 EDT] 786d69800457: (from whenry/testimage:latest) create
    [2014-04-12 18:14:13 -0400 EDT] 786d69800457: (from whenry/testimage:latest) start
    [2014-04-12 18:22:44 -0400 EDT] 786d69800457: (from whenry/testimage:latest) die
    [2014-04-12 18:22:44 -0400 EDT] 786d69800457: (from whenry/testimage:latest) stop
    [2014-04-12 18:23:04 -0400 EDT] 786d69800457: (from whenry/testimage:latest) start
    [2014-04-12 18:23:13 -0400 EDT] 786d69800457: (from whenry/testimage:latest) die
    [2014-04-12 18:23:13 -0400 EDT] 786d69800457: (from whenry/testimage:latest) stop

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.