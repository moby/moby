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

# date "+%B %Y"
date_string = datetime.date.today().strftime('%B %Y')

def print_usage(outtext, docker_cmd, command):
    help = ""
    try:
        #print "RUN ", "".join((docker_cmd, " ", command, " --help"))
        help = subprocess.check_output("".join((docker_cmd, " ", command, " --help")), stderr=subprocess.STDOUT, shell=True)
    except subprocess.CalledProcessError, e:
        help = e.output
    for l in str(help).strip().split("\n"):
        l = l.rstrip()
        if l == '':
            outtext.write("\n")
        else:
            # `docker --help` tells the user the path they called it with
            l = re.sub(docker_cmd, "docker", l)
            outtext.write("    "+l+"\n")
    outtext.write("\n")

# TODO: look for an complain about any missing commands
def update_cli_reference():
    originalFile = "docs/sources/reference/commandline/cli.md"
    os.rename(originalFile, originalFile+".bak")

    intext = open(originalFile+".bak", "r")
    outtext = open(originalFile, "w")

    mode = 'p'
    space = "    "
    command = ""
    # 2 mode line-by line parser
    for line in intext:
        if mode=='p':
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
            #print "CMD ", command
            if not match:
                # The end of the current usage block - Shell out to run docker to see the new output
                print_usage(outtext, docker_cmd, command)
                outtext.write(line)
                mode = 'p'
    if mode == 'c':
        print_usage(outtext, docker_cmd, command)

def update_man_pages():
    cmds = []
    try:
        help = subprocess.check_output("".join((docker_cmd)), stderr=subprocess.STDOUT, shell=True)
    except subprocess.CalledProcessError, e:
        help = e.output
    for l in str(help).strip().split("\n"):
        l = l.rstrip()
        if l != "":
            match = re.match("    (.*?) .*", l)
            if match:
                cmds.append(match.group(1))

    desc_re = re.compile(r".*# DESCRIPTION(.*?)# (OPTIONS|EXAMPLES?).*", re.MULTILINE|re.DOTALL)
    example_re = re.compile(r".*# EXAMPLES?(.*)# HISTORY.*", re.MULTILINE|re.DOTALL)
    history_re = re.compile(r".*# HISTORY(.*)", re.MULTILINE|re.DOTALL)

    for command in cmds:
        print "COMMAND: "+command
        history = ""
        description = ""
        examples = ""
        if os.path.isfile("docs/man/docker-"+command+".1.md"):
            intext = open("docs/man/docker-"+command+".1.md", "r")
            txt = intext.read()
            intext.close()
            match = desc_re.match(txt)
            if match:
                description = match.group(1)
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
        
        help = ""
        try:
            help = subprocess.check_output("".join((docker_cmd, " ", command, " --help")), stderr=subprocess.STDOUT, shell=True)
        except subprocess.CalledProcessError, e:
            help = e.output
        last_key = ""
        for l in str(help).split("\n"):
            l = l.rstrip()
            if l != "":
                match = re.match("Usage: docker "+command+"(.*)", l)
                if match:
                    usage = match.group(1).strip()
                else:
                    #print ">>>>"+l
                    match = re.match("  (-+)(.*) \s+(.*)", l)
                    if match:
                        last_key = match.group(2).rstrip()
                        #print "    found "+match.group(1)
                        key_params[last_key] = match.group(1)+last_key
                        params[last_key] = match.group(3)
                    else:
                        if last_key != "":
                            params[last_key] = params[last_key] + "\n" + l
                        else:
                            if usage_description != "":
                                usage_description = usage_description + "\n"
                            usage_description = usage_description + l
        
        # replace [OPTIONS] with the list of params     
        options = ""
        match = re.match("\[OPTIONS\](.*)", usage)
        if match:
            usage = match.group(1)

        new_usage = ""
        # TODO: sort without the `-`'s
        for key in sorted(params.keys(), key=lambda s: s.lower()):
            # split on commas, remove --?.*=.*, put in *'s mumble
            ps = []
            opts = []
            for k in key_params[key].split(","):
                #print "......"+k
                match = re.match("(-+)([A-Za-z-0-9]*)(?:=(.*))?", k.lstrip())
                if match:
                    p = "**"+match.group(1)+match.group(2)+"**"
                    o = "**"+match.group(1)+match.group(2)+"**"
                    if match.group(3):
                        # if ="" then use UPPERCASE(group(2))"
                        val = match.group(3)
                        if val == "\"\"":
                            val = match.group(2).upper()
                        p = p+"[=*"+val+"*]"
                        val = match.group(3)
                        if val in ("true", "false"):
                            params[key] = params[key].rstrip()
                            if not params[key].endswith('.'):
                                params[key] = params[key]+ "."
                            params[key] = params[key] + " The default is *"+val+"*."
                            val = "*true*|*false*"
                        o = o+"="+val
                    ps.append(p)
                    opts.append(o)
                else:
                    print "nomatch:"+k
            new_usage = new_usage+ "\n["+"|".join(ps)+"]"
            options = options + ", ".join(opts) + "\n   "+ params[key]+"\n\n"
        if new_usage != "":
            new_usage = new_usage.strip() + "\n"
        usage = new_usage + usage
            
        
        outtext = open("docs/man/docker-"+command+".1.md", "w")
        outtext.write("""% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
""")
        outtext.write("docker-"+command+" - "+usage_description+"\n\n")
        outtext.write("# SYNOPSIS\n**docker "+command+"**\n"+usage+"\n\n")
        if description != "":
            outtext.write("# DESCRIPTION"+description)
        if options == "":
            options = "There are no available options.\n\n"
        outtext.write("# OPTIONS\n"+options)
        if examples != "":
           outtext.write("# EXAMPLES"+examples)
        outtext.write("# HISTORY\n")
        if history != "":
           outtext.write(history+"\n")
        recent_history_re = re.compile(".*"+date_string+".*", re.MULTILINE|re.DOTALL)
        if not recent_history_re.match(history):
            outtext.write(date_string+", updated by Sven Dowideit <SvenDowideit@home.org.au>\n")
        outtext.close()

# main
update_cli_reference()
update_man_pages()
