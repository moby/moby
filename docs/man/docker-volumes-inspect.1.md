% DOCKER(1) Docker User Manuals
% Docker Community
% November 2014
# NAME
docker-volumes-inspect - Return low-level information on a volume

# SYNOPSIS
**docker volumes inspect**
[**-f**|**--format**[=*FORMAT*]]
VOLUME [VOLUME...]

# DESCRIPTION

This displays all the information available in Docker for a given
volume. By default, this will render all results in a JSON array. If a format is
specified, the given template will be executed for each result.

# OPTIONS
**-f**, **--format**=""
   Format the output using the given Go template.

# EXAMPLES

## Getting information on a volume

To get information on a volume's name:

    #docker volumes inspect morose_torvalds
{
        "ID": "124670d406cb783e37ad10d339e7f980db75460bcf4fedf1baa00566ebbcad63",
        "Name": "morose_torvalds",
        "Created": "2014-10-01T16:17:01.417634973Z",
        "Path": "/var/lib/docker/vfs/dir/124670d406cb783e37ad10d339e7f980db75460bcf4fedf1baa00566ebbcad63",
        "IsBindMount": false,
        "Writable": true,
        "Containers": [
             "04b22da1ac4c47f758a3553d5c1c36e7510d1e99c80da19504f5f9112bc5491e",
             "835f0b9b62d09145d14339ee2f651df5d8dec1239c3c94b2cdc8a614649ecc1e"
         ],
        "Aliases": [
             "determined_brown:/foo",
             "angry_bell:/bar"
        ]
}

## Getting the host path of a volume

To get the host path of a volume:

    # docker volumes inspect --format='{{.Path}}' morose_torvalds
    /var/lib/docker/vfs/dir/124670d406cb783e37ad10d339e7f980db75460bcf4fedf1baa00566ebbcad63

# HISTORY
November 2014, updated by Brian Goff <cpuguy83@mail.com>
