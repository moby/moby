from behave.log_capture import capture
import distutils.spawn
import logging
import os

import steps.docker


def before_all(context):
    context.docker_binary = "test"
    docker_binary = "docker"
    env_binary = os.getenv(docker_binary)
    docker_binary = env_binary or distutils.spawn.find_executable(docker_binary)
    if docker_binary is None:
        raise "Failed to get path to the docker binary"
    context.docker_binary = docker_binary


def before_scenario(context, scenario):
    context.containers = {}


@capture
def after_scenario(context, scenario):
    # Retrieve the list of containers
    cmd_ps = steps.docker.invoke(context, "ps", "-aq", trace=False)
    stdout, stderr = cmd_ps.communicate()
    if cmd_ps.returncode != 0:
        logging.error("Failed to retrieve container list to cleanup ({})".format(stderr))

    # Make a list of space-separated containers ids
    containers = stdout.replace('\n', ' ').strip()
    if not containers:
        return

    # Kill all containers
    cmd = steps.docker.invoke(context, "kill", containers, trace=False)
    if cmd.wait() != 0:
        logging.error("Error when killing containers ({})".format(cmd.stderr.read()))

    # Remove all containers and their volumes
    cmd = steps.docker.invoke(context, "rm", "-v " + containers, trace=False)
    if cmd.wait() != 0:
        logging.error("Error when removing containers ({})".format(cmd.stderr.read()))

