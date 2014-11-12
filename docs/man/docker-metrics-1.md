% DOCKER(1) Docker User Manuals
% Docker Community
% JANUARY 2015
# NAME
docker-metrics - Show Docker metrics

# SYNOPSIS
**docker metrics**
[**-f**|**--follow**[=*SECONDS*]]


# DESCRIPTION

List the containers in the local repository. By default this show only
the running containers.

# OPTIONS
**-f**, **--follow**="0"
   Update metrics every specified seconds. 0 = disabled.

# EXAMPLES
# Show global Docker metrics.

    # docker metrics
    METRIC                                             VALUE
    http_requests_total                             63251.00
    http_request_duration_seconds/ping                  0.02
    http_request_duration_seconds/getEvents             0.53
    http_request_duration_seconds/getVersion            0.03
    http_request_duration_seconds/getImagesJSON         0.23
    http_request_duration_seconds/getImagesViz          0.25
    http_request_duration_seconds/getImagesSearch       3.56
    ...
    http_request_duration_seconds/postContainersCreate  2.21
    ...
    http_request_duration_seconds/postContainersPause   0.58
    http_request_duration_seconds/postContainersUnpause 0.69
    ...

