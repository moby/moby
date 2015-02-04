from behave import *

import docker


@then('the "{attribute}" attribute of container "{name}" should be "{expected}"')
def step_impl(context, attribute, name, expected):
    conid = context.containers[name].cid
    value = docker.inspect_field(context, conid, attribute)
    assert value == expected, "Bad value for inspect field: expected {}, got {}".format(expected, value)

