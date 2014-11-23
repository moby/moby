#!/usr/bin/python3
#
# docker-ssh: push and pull docker images over ssh
#
# usage:
#   docker-ssh pull [-h] [USER@]HOST[:PORT] IMAGE [IMAGE ...]
#   docker-ssh push [-h] [USER@]HOST[:PORT] IMAGE [IMAGE ...]
#
# This script uses `docker save` `docker load` and a ssh session to transfer
# images from/to a docker daemon running on a remote host.
#
# If possible it calls the `docker save` command with the --exclude option so
# as not to transfer images that are already present in the destination daemon.
#

import argparse, os, re, subprocess, sys, tempfile, time

PROG = "docker-ssh"

class SshSession:
    def __init__(self, user, host, port):
        self.user = user
        self.host = host
        self.port = port
        self.proc = None

    def cmd(self, args=[], opt=()):
        cmd = list(self.basecmd)
        cmd.extend(opt)
        cmd.append(self.host)
        assert isinstance(args, list)
        cmd.extend(args)
        return cmd

    def __enter__(self):
        assert self.proc is None
        self.tmpdir  = tempfile.TemporaryDirectory()
        self.tmpsock = os.path.join(self.tmpdir.__enter__(), "sock")

        self.basecmd = ["ssh", "-o", "ControlPath %s" % self.tmpsock, "-e", "none"]
        if self.user:
            self.basecmd.extend(["-l", self.user])
        if self.port:
            self.basecmd.extend(["-p", self.port])

        self.proc = subprocess.Popen(self.cmd(opt=["-MN"]))
        while not os.path.exists(self.tmpsock):
            time.sleep(0.1)
            if self.proc.poll() is not None:
                die("ssh connection error")

        return self

    def __exit__(self, a, b, c):
        try:
            self.proc.terminate()
            self.proc.wait()
        finally:
            self.tmpdir.cleanup()
            self.tmpdir.__exit__(a, b, c)

def run(cmd, *, input="", **kw):
    print (cmd)
    proc = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, **kw)
    out, err = proc.communicate(input)

    if proc.returncode:
        die("subprocess returned error code %d" % proc.returncode)
    return out

def run_err(cmd, *, input="", **kw):
    proc = subprocess.Popen(cmd, stdin=subprocess.PIPE, stderr=subprocess.PIPE, **kw)
    out, err = proc.communicate(input)
    return err, proc.returncode

def die(msg):
    sys.stderr.write("%s: %s\n" % (PROG, msg))
    sys.exit(1)

def list_images(docker_cmd):
    return run(docker_cmd + ["images", "-q", "--no-trunc", "-a"]).decode().splitlines()

def push(session, images):
    err, code = run_err(["docker", "save"])
    excludes = list_images(session.cmd(["docker"])) if b"--exclude" in err else ()

    
    cmd = "docker save {exclude} {images} | {ssh} docker load".format(
            ssh     = " ".join(map(repr, session.cmd())),
            exclude = " ".join("-e %s" % x for x in excludes),
            images  = " ".join(map(repr, images))
            )
    run(cmd, shell=True)

    

def pull(session, images):
    err, code = run_err(session.cmd(["docker", "save"]))
    excludes = list_images(["docker"]) if b"--exclude" in err else ()

    cmd = "{ssh} docker save {exclude} {images} | tar tv".format(
            ssh     = " ".join(map(repr, session.cmd())),
            exclude = " ".join("-e %s" % x for x in excludes),
            images  = " ".join(map(repr, images))
            )
    run(cmd, shell=True)

def main():

    parser = argparse.ArgumentParser(prog=PROG,
        	    description="pull and push docker images over ssh")

    sub = parser.add_subparsers(dest="command", help="sub-command help")

    
    p = sub.add_parser("push", help="push images to the remote host")
    p.add_argument("remote", metavar="[USER@]HOST[:PORT]",
            help="paramenters to connect to the remote docker daemon")
    p.add_argument("images", metavar="IMAGE", nargs="+",
            help="docker images to be pushed")

    p = sub.add_parser("pull", help="pull images from the remote host")
    p.add_argument("remote", metavar="[USER@]HOST[:PORT]",
            help="paramenters to connect to the remote docker daemon")
    p.add_argument("images", metavar="IMAGE", nargs="+",
            help="docker images to be pulled")


    args = parser.parse_args()

    mo = re.match(r"(?:([^:@/\s]+)@)?([^:@/\s]+)(?::([^:@/\s]+))?$", args.remote, re.I)
    if not mo:
        die("remote host must match: [USER@]HOST[:PORT]")

    args.user, args.host, args.port = mo.groups()

    with SshSession(args.user, args.host, args.port) as session:
        if args.command == "push":
            push(session, args.images)
        elif args.command == "pull":
            pull(session, args.images)

main()

# vim:sw=4:et:sts=4:nosta:
