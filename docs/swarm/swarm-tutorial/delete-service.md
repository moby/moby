<!--[metadata]>
+++
title = "Delete the service"
description = "Remove the service on the Swarm"
keywords = ["tutorial, cluster management, swarm, service"]
[menu.main]
identifier="swarm-tutorial-delete-service"
parent="swarm-tutorial"
weight=19
advisory = "rc"
+++
<![end-metadata]-->

# Delete the service running on the Swarm

The remaining steps in the tutorial don't use the `helloworld` service, so now
you can delete the service from the Swarm.

1. If you haven't already, open a terminal and ssh into the machine where you
run your manager node. For example, the tutorial uses a machine named
`manager1`.

2. Run `docker service remove helloworld` to remove the `helloworld` service.

    ```
    $ docker service rm helloworld
    helloworld
    ```

3. Run `docker service inspect <SERVICE-ID>` to veriy that Swarm removed the
service. The CLI returns a message that the service is not found:

    ```
    $ docker service inspect helloworld
    []
    Error: no such service or task: helloworld
    ```

## What's next?

In the next step of the tutorial, you set up a new service and and apply a
[rolling update](rolling-update.md).

<p style="margin-bottom:300px">&nbsp;</p>
