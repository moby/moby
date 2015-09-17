% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-alias - Manage the aliases

# SYNOPSIS
**docker alias**
[**--help**]
[**-d**|**--delete**[=*ALIAS*]]
[**-e**|**--expand**[=*ALIAS*]]
[**-l**|**--list**]
[ALIAS COMMANDS]

# DESCRIPTION

Creates, expands, deletes and list the aliases stored in the docker configuration file.

When the command is used with no flags, it stores the new alias (first arguments) and the command (the remaining arguments) in the docker configuration file. 

If the first command of the alias starts with a '!', the execution of the alias will be detached in a separate 'sh'. Otherwise the 'docker alias' will simply expand the alias before delegating to the docker engine. 

The various flags below are exclusive.

# OPTIONS
**--help**
  Print usage statement

**-d**, **--delete**=""
   Deletes the specified alias from the docker configuration file.

**-e**, **--expand**=""
   Output the command attached with the specified alias.

**-l**, **--list**
   List all the existing alias.


# EXAMPLES

## Manage aliases

Here is an example of creating, using and deleting aliases.

    $ docker alias last ps -l -q
    alias last has been updated
    $ docker alias gc '!f() { docker rm $(docker ps -a -q) ; } ; f'
    alias gc has been updated
    $ docker last
	36e42d968042
    $ docker gc
	36e42d968042
	2a7089024734
    $ docker alias
    alias gc=!f() { docker rm $(docker ps -a -q) ; } ; f
    alias last=ps -l -q
    $ docker alias --delete last
    Alias last has been deleted

## Aliases examples

Here is a list of what could be easily achieved with aliases:

	: remove stopped containers
    docker alias clean '!f() { docker rm $(docker ps -a -q) ; } ; f'
    
    : returns network information of a container (requires jq)
    docker alias network '!f() { docker inspect $1 | jq ".[0].NetworkSettings | {ip: .IPAddress, ports: .Ports}" ; } ; f'
    
    : start a bash on a running container
    docker alias join '!f() { docker exec -it $1 /bin/bash ; } ; f'
    
    : hello world !
    docker alias hello run --rm docker/whalesay cowsay hello world
    
    : return the last container id
    docker alias last ps -l -q
    
    : starts your favorite container and join
    docker alias ubuntu run -i -t --rm ubuntu /bin/bash
    
    : go on strike and shutdown docker
    docker alias onstrike '!f() { sudo service docker stop ; sudo service docker status ; } ; f'
    
    : go at work and start docker
    docker alias atwork '!f() { sudo service docker start ; sudo service docker status ; } ; f' 

# HISTORY
Originally created by Mathieu POUSSE, Armel GOURIOU and Guillaume GERBAUD during the global hack day #3
