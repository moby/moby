#!/usr/bin/env python

#
# Sven's quick hack script to update the documentation
#
# call with:
#       ./docs/update.py /usr/bin/docker
#

import datetime
import re
from sys import argv
import subprocess
import os
import os.path

script, docker_cmd = argv

date_string = datetime.date.today().strftime('%B %Y')


def print_usage(outtext, docker_cmd, command):
    try:
        help_string = subprocess.check_output(
            "".join((docker_cmd, " ", command, " --help")),
            stderr=subprocess.STDOUT,
            shell=True
        )
    except subprocess.CalledProcessError, e:
        help_string = e.output
    for l in str(help_string).strip().split("\n"):
        l = l.rstrip()
        if l == '':
            outtext.write("\n")
        else:
            # `docker --help` tells the user the path they called it with
            l = re.sub(docker_cmd, "docker", l)
            outtext.write("    {}\n".format(l))
    outtext.write("\n")


# TODO: look for an complain about any missing commands
def update_cli_reference():
    originalFile = "docs/sources/reference/commandline/cli.md"
    os.rename(originalFile, originalFile+".bak")

    intext = open("{}.bak".format(originalFile), "r")
    outtext = open(originalFile, "w")

    mode = 'p'
    space = "    "
    command = ""
    # 2 mode line-by line parser
    for line in intext:
        if mode == 'p':
            # Prose
            match = re.match("(    \s*)Usage: docker ([a-z]+)", line)
            if match:
                # the begining of a Docker command usage block
                space = match.group(1)
                command = match.group(2)
                mode = 'c'
            else:
                match = re.match("(    \s*)Usage of .*docker.*:", line)
                if match:
                    # the begining of the Docker --help usage block
                    space = match.group(1)
                    command = ""
                    mode = 'c'
                else:
                    outtext.write(line)
        else:
            # command usage block
            match = re.match("("+space+")(.*)|^$", line)
            if not match:
                # The end of the current usage block
                # Shell out to run docker to see the new output
                print_usage(outtext, docker_cmd, command)
                outtext.write(line)
                mode = 'p'
    if mode == 'c':
        print_usage(outtext, docker_cmd, command)


def update_man_pages():
    cmds = []
    try:
        help_string = subprocess.check_output(
            "".join((docker_cmd)),
            stderr=subprocess.STDOUT,
            shell=True
        )
    except subprocess.CalledProcessError, e:
        help_string = e.output
    for l in str(help_string).strip().split("\n"):
        l = l.rstrip()
        if l != "":
            match = re.match("    (.*?) .*", l)
            if match:
                cmds.append(match.group(1))

    desc_re = re.compile(
        r".*# DESCRIPTION(.*?)# (OPTIONS|EXAMPLES?).*",
        re.MULTILINE | re.DOTALL
    )

    options_re = re.compile(
        r".*# OPTIONS(.*?)# (HISTORY|EXAMPLES?).*",
        re.MULTILINE | re.DOTALL
    )

    example_re = re.compile(
        r".*# EXAMPLES?(.*)# HISTORY.*",
        re.MULTILINE | re.DOTALL
    )

    history_re = re.compile(
        r".*# HISTORY(.*)",
        re.MULTILINE | re.DOTALL
    )

    for command in cmds:
        print "COMMAND: "+command
        if command == "":
            print "SKIPPING"
            continue
        history = ""
        description = ""
        original_options = ""
        examples = ""
        if os.path.isfile("docs/man/docker-"+command+".1.md"):
            intext = open("docs/man/docker-"+command+".1.md", "r")
            txt = intext.read()
            intext.close()
            match = desc_re.match(txt)
            if match:
                description = match.group(1)
            match = options_re.match(txt)
            if match:
                original_options = match.group(1)
		#print "MATCHED OPTIONS\n" + original_options
            match = example_re.match(txt)
            if match:
                examples = match.group(1)
            match = history_re.match(txt)
            if match:
                history = match.group(1).strip()

        usage = ""
        usage_description = ""
        params = {}
        key_params = {}

        try:
            help_string = subprocess.check_output(
                "".join((docker_cmd, " ", command, " --help")),
                stderr=subprocess.STDOUT,
                shell=True
            )
        except subprocess.CalledProcessError, e:
            help_string = e.output

        last_key = ""
        for l in str(help_string).split("\n"):
            l = l.rstrip()
            if l != "":
                match = re.match("Usage: docker {}(.*)".format(command), l)
                if match:
                    usage = match.group(1).strip()
                else:
                    match = re.match("  (-+)(.*) \s+(.*)", l)
                    if match:
                        last_key = match.group(2).rstrip()
                        key_params[last_key] = match.group(1)+last_key
                        params[last_key] = match.group(3)
                    else:
                        if last_key != "":
                            params[last_key] = "{}\n{}".format(params[last_key], l)
                        else:
                            if usage_description != "":
                                usage_description = usage_description + "\n"
                            usage_description = usage_description + l

        # replace [OPTIONS] with the list of params
        options = ""
        match = re.match("\[OPTIONS\]\s*(.*)", usage)
        if match:
            usage = match.group(1)

        new_usage = ""
        # TODO: sort without the `-`'s
        for key in sorted(params.keys(), key=lambda s: s.lower()):
            # split on commas, remove --?.*=.*, put in *'s mumble
            flags = []
            ps = []
            opts = []
            for k in key_params[key].split(","):
                match = re.match("(-+)([A-Za-z-0-9]*)(?:=(.*))?", k.lstrip())
                if match:
                    flags.append("{}{}".format(match.group(1), match.group(2)))
                    p = "**{}{}**".format(match.group(1), match.group(2))
                    o = "**{}{}**".format(match.group(1), match.group(2))
                    if match.group(3):
                        val = match.group(3)
                        if val == "\"\"":
                            val = match.group(2).upper()
                        p = "{}[=*{}*]".format(p, val)
                        val = match.group(3)
                        if val in ("true", "false"):
                            params[key] = params[key].rstrip()
                            if not params[key].endswith('.'):
                                params[key] = params[key]+ "."
                            params[key] = "{} The default is *{}*.".format(params[key], val)
                            val = "*true*|*false*"
                        o = "{}={}".format(o, val)
                    ps.append(p)
                    opts.append(o)
                else:
                    print "nomatch:{}".format(k)
            new_usage = "{}\n[{}]".format(new_usage, "|".join(ps))

            options = "{}{}\n   {}\n\n".format(options, ", ".join(opts), params[key])

            # look at the original options documentation and if its hand written, add it too.
            print "SVEN_re: "+flags[0]
            singleoption_re = re.compile(
                r".*[\r\n]\*\*"+flags[0]+"\*\*([^\r\n]*)[\r\n]+(.*?)[\r\n](\*\*-|# [A-Z]|\*\*[A-Z]+\*\*).*",
                #r""+flags[0]+"(.*)(^\*\*-.*)?",
                re.MULTILINE | re.DOTALL
            )
            match = singleoption_re.match(original_options)
            if match:
                info = match.group(2).strip()
                print "MATCHED: " + match.group(1).strip()
                if info != params[key].strip():
                    #info = re.sub(params[key].strip(), '', info, flags=re.MULTILINE)
                    print "INFO changed: " +info
                    options = "{}   {}\n\n".format(options, info.strip())

        if new_usage != "":
            new_usage = "{}\n".format(new_usage.strip())
        usage = new_usage + usage

        outtext = open("docs/man/docker-{}.1.md".format(command), "w")
        outtext.write("""% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
""")
        outtext.write("docker-{} - {}\n\n".format(command, usage_description))
        outtext.write("# SYNOPSIS\n**docker {}**\n{}\n\n".format(command, usage))
        if description != "":
            outtext.write("# DESCRIPTION{}".format(description))
        if options == "":
            options = "There are no available options.\n\n"
        outtext.write("# OPTIONS\n{}".format(options))
        if examples != "":
           outtext.write("# EXAMPLES{}".format(examples))
        outtext.write("# HISTORY\n")
        if history != "":
           outtext.write("{}\n".format(history))
        recent_history_re = re.compile(
            ".*{}.*".format(date_string),
            re.MULTILINE | re.DOTALL
        )
#        if not recent_history_re.match(history):
#            outtext.write("{}, updated by Sven Dowideit <SvenDowideit@home.org.au>\n".format(date_string))
        outtext.close()

# main
update_cli_reference()
update_man_pages()
