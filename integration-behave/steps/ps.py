from behave import *
import itertools

import docker
import table


@then('the {column} column of ps output for container "{id_or_name}" is "{value}"')
def step_impl(context, column, id_or_name, value):
    # Run `docker ps -a`.
    cmd = docker.invoke(context, "ps", "-a")
    stdout, stderr = cmd.communicate()
    assert cmd.returncode == 0, \
        "Failed to invoke docker ps (code: {}, output: {}".format(cmd.returncode, stdout)

    # Parse command output and find the matching container.
    structd_output = table.parse(stdout)
    cont_predicate = lambda c: c["CONTAINER ID"] == id_or_name[:8] or c["NAMES"] == id_or_name
    find_container = next(itertools.ifilter(cont_predicate, structd_output), None)
    assert find_container is not None, \
         'No container with ID or name {} in ps output'.format(id_or_name)
    assert find_container[column] == value, \
        'Wrong value in column "{}" for container {}: expected {}, got {}'.format(column, id_or_name, value, find_container[column])

