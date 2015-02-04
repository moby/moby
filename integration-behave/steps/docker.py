import logging
import subprocess
import shlex


def invoke(context, command, args='', trace=True):
    args = [context.docker_binary, command] + shlex.split(args)
    if trace:
        logging.info("Invoking command: {}".format(args))

    return subprocess.Popen(
            args,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            universal_newlines=True
        )

def inspect_field(context, container, field, **kwargs):
    args = "-f '{{{{ .{} }}}}' {}".format(field, container)
    cmd = invoke(context, "inspect", args, **kwargs)
    stdout, stderr = cmd.communicate()
    return stdout and stdout.rstrip()


def run(context, name, args):
    # That's a bit of a hack, but we need to know whether the container was
    # launched detached or not: when it is we read the id from stdout, when it
    # is not we inject a --name flag if necessary.
    sh_args = shlex.split(args)
    detached = "-d" in sh_args
    if not detached and "--name" not in sh_args:
        args = "--name {} {}".format(name, args)
        sh_args = shlex.split(args)

    # Execute the command and retrieve the container id
    cmd = invoke(context, "run", args)
    if not detached:
        cid = sh_args[sh_args.index("--name") + 1]
    else:
        cid, _ = cmd.communicate()

    # Return the process and container id
    return cmd, cid.strip()
