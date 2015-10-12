# Experimental: User namespace support

Linux kernel [user namespace support](http://man7.org/linux/man-pages/man7/user_namespaces.7.html) provides additional security by enabling
a process--and therefore a container--to have a unique range of user and
group IDs which are outside the traditional user and group range utilized by
the host system. Potentially the most important security improvement is that,
by default, container processes running as the `root` user will have expected
administrative privilege (with some restrictions) inside the container but will
effectively be mapped to an unprivileged `uid` on the host.

In this experimental phase, the Docker daemon creates a single daemon-wide mapping
for all containers running on the same engine instance. The mappings will
utilize the existing subordinate user and group ID feature available on all modern
Linux distributions.
The [`/etc/subuid`](http://man7.org/linux/man-pages/man5/subuid.5.html) and 
[`/etc/subgid`](http://man7.org/linux/man-pages/man5/subgid.5.html) files will be
read for the user, and optional group, specified to the `--userns-remap` 
parameter.  If you do not wish to specify your own user and/or group, you can 
provide `default` as the value to this flag, and a user will be created on your behalf
and provided subordinate uid and gid ranges. This default user will be named
`dockremap`, and entries will be created for it in `/etc/passwd` and 
`/etc/group` using your distro's standard user and group creation tools.

> **Note**: The single mapping per-daemon restriction exists for this experimental
> phase because Docker shares image layers from its local cache across all
> containers running on the engine instance.  Since file ownership must be
> the same for all containers sharing the same layer content, the decision
> was made to map the file ownership on `docker pull` to the daemon's user and
> group mappings so that there is no delay for running containers once the
> content is downloaded--exactly the same performance characteristics as with
> user namespaces disabled.

## Starting the daemon with user namespaces enabled
To enable this experimental user namespace support for a Docker daemon instance,
start the daemon with the aforementioned `--userns-remap` flag, which accepts
values in the following formats:

 - uid
 - uid:gid
 - username
 - username:groupname

If numeric IDs are provided, translation back to valid user or group names
will occur so that the subordinate uid and gid information can be read, given
these resources are name-based, not id-based.  If the numeric ID information
provided does not exist as entries in `/etc/passwd` or `/etc/group`, dameon
startup will fail with an error message.

*An example: starting with default Docker user management:*

```
     $ docker daemon --userns-remap=default
```    
In this case, Docker will create--or find the existing--user and group
named `dockremap`. If the user is created, and the Linux distribution has
appropriate support, the `/etc/subuid` and `/etc/subgid` files will be populated
with a contiguous 65536 length range of subordinate user and group IDs, starting
at an offset based on prior entries in those files.  For example, Ubuntu will
create the following range, based on an existing user already having the first
65536 range:

```
     $ cat /etc/subuid
     user1:100000:65536
     dockremap:165536:65536
```

> **Note:** On a fresh Fedora install, we found that we had to `touch` the
> `/etc/subuid` and `/etc/subgid` files to have ranges assigned when users
> were created.  Once these files existed, range assigment on user creation
> worked properly.

If you have a preferred/self-managed user with subordinate ID mappings already
configured, you can provide that username or uid to the `--userns-remap` flag.
If you have a group that doesn't match the username, you may provide the `gid`
or group name as well; otherwise the username will be used as the group name
when querying the system for the subordinate group ID range.

## Detailed information on `subuid`/`subgid` ranges

Given there may be advanced use of the subordinate ID ranges by power users, we will
describe how the Docker daemon uses the range entries within these files under the
current experimental user namespace support.

The simplest case exists where only one contiguous range is defined for the
provided user or group. In this case, Docker will use that entire contiguous
range for the mapping of host uids and gids to the container process.  This 
means that the first ID in the range will be the remapped root user, and the
IDs above that initial ID will map host ID 1 through the end of the range.

From the example `/etc/subid` content shown above, that means the remapped root
user would be uid 165536.

If the system administrator has set up multiple ranges for a single user or
group, the Docker daemon will read all the available ranges and use the
following algorithm to create the mapping ranges:

1. The ranges will be sorted by *start ID* ascending
2. Maps will be created from each range with where the host ID will increment starting at 0 for the first range, 0+*range1* length for the second, and so on.  This means that the lowest range start ID will be the remapped root, and all further ranges will map IDs from 1 through the uid or gid that equals the sum of all range lengths.
3. Ranges segments above five will be ignored as the kernel ignores any ID maps after five (in `/proc/self/{u,g}id_map`)

## User namespace known restrictions

The following standard Docker features are currently incompatible when
running a Docker daemon with experimental user namespaces enabled:

 - sharing namespaces with the host (--pid=host, --net=host, etc.)
 - sharing namespaces with other containers (--net=container:*other*)
 - A `--readonly` container filesystem (a Linux kernel restriction on remount with new flags of a currently mounted filesystem when inside a user namespace)
 - external (volume/graph) drivers which are unaware/incapable of using daemon user mappings
 - Using `--privileged` mode containers
 - Using the lxc execdriver (only the `native` execdriver is enabled to use user namespaces)
 - volume use without pre-arranging proper file ownership in mounted volumes

Additionally, while the `root` user inside a user namespaced container
process has many of the privileges of the administrative root user, the
following operations will fail:

 - Use of `mknod` - permission is denied for device creation by the container root
 - others will be listed here when fully tested
