% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-top - Display the running processes of a container

# SYNOPSIS
**docker top**
CONTAINER [ps FIELDS]

# DESCRIPTION

Look up the running process of the container. FIELDS can be a space-separated
list of fields desired in the output. Valid fields are:

**c** - integer
   Processor utilization over process lifetime

**comm** - string
   Command name (only the executable name)

**command** - string
   Command with all its arguments

**cputime** - number
   Cumulative CPU time for the process in seconds, with or without decimals

**gid** - integer
   Effective group ID number of the process as a decimal integer

**lwp** - integer
   Light weight process (thread) ID of the dispatchable entity

**nice** - integer
   Nice value. This ranges from 19 (nicest) to -20 (not nice to others)

**pcpu** - number
   Cpu utilization of the process as a percentage of its run time, with or without decimals

**pid** - integer
   A number representing the process ID

**pgid** - integer
   Process group ID or, equivalently, the process ID of the process group leader

**pmem** - integer
   Ratio of the process's resident set size to the physical memory on the machine, expressed as a percentage

**ppid** - integer
   Parent process ID

**psr** - integer
   Processor that process is currently assigned to

**rgid** - integer
   Real group ID

**rss** - integer
   Resident set size, the non-swapped physical memory that a task has used in KiB

**ruid** - integer
   Real user ID

**start_time** - number
   Time the process started (seconds since 1970-01-01 00:00:00 UTC), with or without decimals

**state** - string (see `state` below)
   Normalized state of the process

**state_flags** - string (see `state flags` below)
   Implementation-specific process state flags

**tty** - string
   Controlling tty (terminal)

**uid** - integer
   Effective user ID

**vsz** - integer
   Virtual size of the process in KiB

# state

The state field when requested will be one of the following:

**uninterruptible**
   Uninterruptible sleep (usually IO)

**running**
   Running or runnable (on run queue)

**sleep**
   Interruptible sleep (waiting for an event to complete)

**stopped**
   Stopped, either by a job control signal or because it is being traced

**zombie**
   Defunct ("zombie") process, terminated but not reaped by its parent


# state flags

The state_flags field when requested will be either an empty string or a
comma-separated combination of the following values:

**high**
   High-priority (not nice to other users)

**low**
   Low-priority (nice to other users)

**locked**
   Has pages locked into memory (for real-time and custom IO)

**leader**
   Is a session leader

**threads**
   Is multi-threaded (using CLONE_THREAD, like NPTL pthreads do)

**foreground**
   Is in the foreground process group


# OPTIONS
There are no available options.

# EXAMPLES

Run **docker top** with the fields 'pid tty state state_flags command':

    $ sudo docker top 8601afda2b pid tty state state_flags cpu_time command
    pid      tty       state    state_flags   cpu_time    command
    16623    ?         S        leader        0.03        sleep 99999


# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
