from behave import *
import shlex
import time

import docker


@given('an empty environment')
def step_impl(context):
    pass


@given('a container "{name}" started with "{args}"')
def step_impl(context, name, args):
    cmd, cid = docker.run(context, name, args)
    assert not cmd.poll(), "Error launching container (exit code: {})".format(cmd.returncode)
    context.containers[name] = cmd
    context.containers[name].cid = cid


@when('I start container "{name}" with "{args}"')
def step_impl(context, name, args):
    cmd, cid = docker.run(context, name, args)
    assert not cmd.poll(), "Error launching container (exit code: {})".format(cmd.returncode)
    context.containers[name] = cmd
    context.containers[name].cid = cid


@when('I send "{data}" to container "{name}"')
def step_impl(context, data, name):
    assert name in context.containers, 'Unknown container "{}"'.format(name)
    context.containers[name].stdin.write(data)


@when('I wait for the container "{name}" to be running')
def step_impl(context, name):
    assert name in context.containers, 'Unknown container "{}"'.format(name)
    for _ in range(10):
        if docker.inspect_field(context, name, "State.Running", trace=False) == "true":
            return
        time.sleep(0.1)
    assert False, "Container didn't start in time"


@then('the container "{name}" stops')
def step_impl(context, name):
    assert name in context.containers, 'Unknown container "{}"'.format(name)
    for _ in range(10):
        if docker.inspect_field(context, name, "State.Running", trace=False) == "false":
            return
        time.sleep(0.1)
    assert False, "Container didn't stop in time"


@then('the container "{name}" outputs "{expected}"')
def step_impl(context, name, expected):
    assert name in context.containers, 'Unknown container "{}"'.format(name)
    import time
    time.sleep(1)
    stdout = context.containers[name].stdout.readline()
    assert stdout == expected, 'Unexpected output: expected "{}", got "{}"'.format(expected, stdout)

