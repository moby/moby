<!--[metadata]>
+++
title = "events"
description = "The events command description and usage"
keywords = ["events, container, report"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# events

    Usage: docker events [OPTIONS]

    Get real time events from the server

      -f, --filter=[]    Filter output based on conditions provided
      --help             Print usage
      --since=""         Show all events created since timestamp
      --until=""         Stream events until this timestamp

Docker containers report the following events:

    attach, commit, copy, create, destroy, detach, die, exec_create, exec_detach, exec_start, export, kill, oom, pause, rename, resize, restart, start, stop, top, unpause, update

Docker images report the following events:

    delete, import, load, pull, push, save, tag, untag

Docker volumes report the following events:

    create, mount, unmount, destroy

Docker networks report the following events:

    create, connect, disconnect, destroy

Docker daemon report the following events:

    reload

The `--since` and `--until` parameters can be Unix timestamps, date formatted
timestamps, or Go duration strings (e.g. `10m`, `1h30m`) computed
relative to the client machine’s time. If you do not provide the `--since` option,
the command returns only new and/or live events.  Supported formats for date
formatted time stamps include RFC3339Nano, RFC3339, `2006-01-02T15:04:05`,
`2006-01-02T15:04:05.999999999`, `2006-01-02Z07:00`, and `2006-01-02`. The local
timezone on the client will be used if you do not provide either a `Z` or a
`+-00:00` timezone offset at the end of the timestamp.  When providing Unix
timestamps enter seconds[.nanoseconds], where seconds is the number of seconds
that have elapsed since January 1, 1970 (midnight UTC/GMT), not counting leap
seconds (aka Unix epoch or Unix time), and the optional .nanoseconds field is a
fraction of a second no more than nine digits long.

## Filtering

The filtering flag (`-f` or `--filter`) format is of "key=value". If you would
like to use multiple filters, pass multiple flags (e.g.,
`--filter "foo=bar" --filter "bif=baz"`)

Using the same filter multiple times will be handled as a *OR*; for example
`--filter container=588a23dac085 --filter container=a8f7720b8c22` will display
events for container 588a23dac085 *OR* container a8f7720b8c22

Using multiple filters will be handled as a *AND*; for example
`--filter container=588a23dac085 --filter event=start` will display events for
container container 588a23dac085 *AND* the event type is *start*

The currently supported filters are:

* container (`container=<name or id>`)
* event (`event=<event action>`)
* image (`image=<tag or id>`)
* label (`label=<key>` or `label=<key>=<value>`)
* type (`type=<container or image or volume or network or daemon>`)
* volume (`volume=<name or id>`)
* network (`network=<name or id>`)
* daemon (`daemon=<name or id>`)

## Examples

You'll need two shells for this example.

**Shell 1: Listening for events:**

    $ docker events

**Shell 2: Start and Stop containers:**

    $ docker start 4386fb97867d
    $ docker stop 4386fb97867d
    $ docker stop 7805c1d35632

**Shell 1: (Again .. now showing events):**

    2015-05-12T11:51:30.999999999Z07:00 container start 4386fb97867d (image=ubuntu-1:14.04)
    2015-05-12T11:51:30.999999999Z07:00 container die 4386fb97867d (image=ubuntu-1:14.04)
    2015-05-12T15:52:12.999999999Z07:00 container stop 4386fb97867d (image=ubuntu-1:14.04)
    2015-05-12T15:53:45.999999999Z07:00 container die 7805c1d35632 (image=redis:2.8)
    2015-05-12T15:54:03.999999999Z07:00 container stop 7805c1d35632 (image=redis:2.8)

**Show events in the past from a specified time:**

    $ docker events --since 1378216169
    2015-05-12T11:51:30.999999999Z07:00 container die 4386fb97867d (image=ubuntu-1:14.04)
    2015-05-12T15:52:12.999999999Z07:00 container stop 4386fb97867d (image=ubuntu-1:14.04)
    2015-05-12T15:53:45.999999999Z07:00 container die 7805c1d35632 (image=redis:2.8)
    2015-05-12T15:54:03.999999999Z07:00 container stop 7805c1d35632 (image=redis:2.8)

    $ docker events --since '2013-09-03'
    2015-05-12T11:51:30.999999999Z07:00 container start 4386fb97867d (image=ubuntu-1:14.04)
    2015-05-12T11:51:30.999999999Z07:00 container die 4386fb97867d (image=ubuntu-1:14.04)
    2015-05-12T15:52:12.999999999Z07:00 container stop 4386fb97867d (image=ubuntu-1:14.04)
    2015-05-12T15:53:45.999999999Z07:00 container die 7805c1d35632 (image=redis:2.8)
    2015-05-12T15:54:03.999999999Z07:00 container stop 7805c1d35632 (image=redis:2.8)

    $ docker events --since '2013-09-03T15:49:29'
    2015-05-12T11:51:30.999999999Z07:00 container die 4386fb97867d (image=ubuntu-1:14.04)
    2015-05-12T15:52:12.999999999Z07:00 container stop 4386fb97867d (image=ubuntu-1:14.04)
    2015-05-12T15:53:45.999999999Z07:00 container die 7805c1d35632 (image=redis:2.8)
    2015-05-12T15:54:03.999999999Z07:00 container stop 7805c1d35632 (image=redis:2.8)

This example outputs all events that were generated in the last 3 minutes,
relative to the current time on the client machine:

    $ docker events --since '3m'
    2015-05-12T11:51:30.999999999Z07:00 container die 4386fb97867d (image=ubuntu-1:14.04)
    2015-05-12T15:52:12.999999999Z07:00 container stop 4386fb97867d (image=ubuntu-1:14.04)
    2015-05-12T15:53:45.999999999Z07:00 container die 7805c1d35632 (image=redis:2.8)
    2015-05-12T15:54:03.999999999Z07:00 container stop 7805c1d35632 (image=redis:2.8)

**Filter events:**

    $ docker events --filter 'event=stop'
    2014-05-10T17:42:14.999999999Z07:00 container stop 4386fb97867d (image=ubuntu-1:14.04)
    2014-09-03T17:42:14.999999999Z07:00 container stop 7805c1d35632 (image=redis:2.8)

    $ docker events --filter 'image=ubuntu-1:14.04'
    2014-05-10T17:42:14.999999999Z07:00 container start 4386fb97867d (image=ubuntu-1:14.04)
    2014-05-10T17:42:14.999999999Z07:00 container die 4386fb97867d (image=ubuntu-1:14.04)
    2014-05-10T17:42:14.999999999Z07:00 container stop 4386fb97867d (image=ubuntu-1:14.04)

    $ docker events --filter 'container=7805c1d35632'
    2014-05-10T17:42:14.999999999Z07:00 container die 7805c1d35632 (image=redis:2.8)
    2014-09-03T15:49:29.999999999Z07:00 container stop 7805c1d35632 (image= redis:2.8)

    $ docker events --filter 'container=7805c1d35632' --filter 'container=4386fb97867d'
    2014-09-03T15:49:29.999999999Z07:00 container die 4386fb97867d (image=ubuntu-1:14.04)
    2014-05-10T17:42:14.999999999Z07:00 container stop 4386fb97867d (image=ubuntu-1:14.04)
    2014-05-10T17:42:14.999999999Z07:00 container die 7805c1d35632 (image=redis:2.8)
    2014-09-03T15:49:29.999999999Z07:00 container stop 7805c1d35632 (image=redis:2.8)

    $ docker events --filter 'container=7805c1d35632' --filter 'event=stop'
    2014-09-03T15:49:29.999999999Z07:00 container stop 7805c1d35632 (image=redis:2.8)

    $ docker events --filter 'container=container_1' --filter 'container=container_2'
    2014-09-03T15:49:29.999999999Z07:00 container die 4386fb97867d (image=ubuntu-1:14.04)
    2014-05-10T17:42:14.999999999Z07:00 container stop 4386fb97867d (image=ubuntu-1:14.04)
    2014-05-10T17:42:14.999999999Z07:00 container die 7805c1d35632 (imager=redis:2.8)
    2014-09-03T15:49:29.999999999Z07:00 container stop 7805c1d35632 (image=redis:2.8)

    $ docker events --filter 'type=volume'
    2015-12-23T21:05:28.136212689Z volume create test-event-volume-local (driver=local)
    2015-12-23T21:05:28.383462717Z volume mount test-event-volume-local (read/write=true, container=562fe10671e9273da25eed36cdce26159085ac7ee6707105fd534866340a5025, destination=/foo, driver=local, propagation=rprivate)
    2015-12-23T21:05:28.650314265Z volume unmount test-event-volume-local (container=562fe10671e9273da25eed36cdce26159085ac7ee6707105fd534866340a5025, driver=local)
    2015-12-23T21:05:28.716218405Z volume destroy test-event-volume-local (driver=local)

    $ docker events --filter 'type=network'
    2015-12-23T21:38:24.705709133Z network create 8b111217944ba0ba844a65b13efcd57dc494932ee2527577758f939315ba2c5b (name=test-event-network-local, type=bridge)
    2015-12-23T21:38:25.119625123Z network connect 8b111217944ba0ba844a65b13efcd57dc494932ee2527577758f939315ba2c5b (name=test-event-network-local, container=b4be644031a3d90b400f88ab3d4bdf4dc23adb250e696b6328b85441abe2c54e, type=bridge)
