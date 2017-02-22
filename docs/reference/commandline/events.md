---
title: "events"
description: "The events command description and usage"
keywords: "events, container, report"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# events

```markdown
Usage:  docker events [OPTIONS]

Get real time events from the server

Options:
  -f, --filter value   Filter output based on conditions provided (default [])
      --format string  Format the output using the given Go template
      --help           Print usage
      --since string   Show all events created since timestamp
      --until string   Stream events until this timestamp
```

## Description

Use `docker events` to get real-time events from the server. These events differ
per Docker object type.

### Object types

#### Containers

Docker containers report the following events:

- `attach`
- `commit`
- `copy`
- `create`
- `destroy`
- `detach`
- `die`
- `exec_create`
- `exec_detach`
- `exec_start`
- `export`
- `health_status`
- `kill`
- `oom`
- `pause`
- `rename`
- `resize`
- `restart`
- `start`
- `stop`
- `top`
- `unpause`
- `update`

#### Images

Docker images report the following events:

- `delete`
- `import`
- `load`
- `pull`
- `push`
- `save`
- `tag`
- `untag`

#### Plugins

Docker plugins report the following events:

- `install`
- `enable`
- `disable`
- `remove`

#### Volumes

Docker volumes report the following events:

- `create`
- `mount`
- `unmount`
- `destroy`

#### Networks

Docker networks report the following events:

- `create`
- `connect`
- `disconnect`
- `destroy`

#### Daemons

Docker daemons report the following events:

- `reload`

### Limiting, filtering, and formatting the output

#### Limit events by time

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

#### Filtering

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
* daemon (`daemon=<name or id>`)
* event (`event=<event action>`)
* image (`image=<tag or id>`)
* label (`label=<key>` or `label=<key>=<value>`)
* network (`network=<name or id>`)
* plugin (`plugin=<name or id>`)
* type (`type=<container or image or volume or network or daemon>`)
* volume (`volume=<name or id>`)

#### Format

If a format (`--format`) is specified, the given template will be executed
instead of the default
format. Go's [text/template](http://golang.org/pkg/text/template/) package
describes all the details of the format.

If a format is set to `{{json .}}`, the events are streamed as valid JSON
Lines. For information about JSON Lines, please refer to http://jsonlines.org/ .

## Examples

### Basic example

You'll need two shells for this example.

**Shell 1: Listening for events:**

```bash
$ docker events
```

**Shell 2: Start and Stop containers:**

```bash
$ docker create --name test alpine:latest top
$ docker start test
$ docker stop test
```

**Shell 1: (Again .. now showing events):**

```none
2017-01-05T00:35:58.859401177+08:00 container create 0fdb48addc82871eb34eb23a847cfd033dedd1a0a37bef2e6d9eb3870fc7ff37 (image=alpine:latest, name=test)
2017-01-05T00:36:04.703631903+08:00 network connect e2e1f5ceda09d4300f3a846f0acfaa9a8bb0d89e775eb744c5acecd60e0529e2 (container=0fdb...ff37, name=bridge, type=bridge)
2017-01-05T00:36:04.795031609+08:00 container start 0fdb...ff37 (image=alpine:latest, name=test)
2017-01-05T00:36:09.830268747+08:00 container kill 0fdb...ff37 (image=alpine:latest, name=test, signal=15)
2017-01-05T00:36:09.840186338+08:00 container die 0fdb...ff37 (exitCode=143, image=alpine:latest, name=test)
2017-01-05T00:36:09.880113663+08:00 network disconnect e2e...29e2 (container=0fdb...ff37, name=bridge, type=bridge)
2017-01-05T00:36:09.890214053+08:00 container stop 0fdb...ff37 (image=alpine:latest, name=test)
```

To exit the `docker events` command, use `CTRL+C`.

### Filter events by time

You can filter the output by an absolute timestamp or relative time on the host
machine, using the following different time syntaxes:

```bash
$ docker events --since 1483283804
2017-01-05T00:35:41.241772953+08:00 volume create testVol (driver=local)
2017-01-05T00:35:58.859401177+08:00 container create d9cd...4d70 (image=alpine:latest, name=test)
2017-01-05T00:36:04.703631903+08:00 network connect e2e1...29e2 (container=0fdb...ff37, name=bridge, type=bridge)
2017-01-05T00:36:04.795031609+08:00 container start 0fdb...ff37 (image=alpine:latest, name=test)
2017-01-05T00:36:09.830268747+08:00 container kill 0fdb...ff37 (image=alpine:latest, name=test, signal=15)
2017-01-05T00:36:09.840186338+08:00 container die 0fdb...ff37 (exitCode=143, image=alpine:latest, name=test)
2017-01-05T00:36:09.880113663+08:00 network disconnect e2e...29e2 (container=0fdb...ff37, name=bridge, type=bridge)
2017-01-05T00:36:09.890214053+08:00 container stop 0fdb...ff37 (image=alpine:latest, name=test)

$ docker events --since '2017-01-05'
2017-01-05T00:35:41.241772953+08:00 volume create testVol (driver=local)
2017-01-05T00:35:58.859401177+08:00 container create d9cd...4d70 (image=alpine:latest, name=test)
2017-01-05T00:36:04.703631903+08:00 network connect e2e1...29e2 (container=0fdb...ff37, name=bridge, type=bridge)
2017-01-05T00:36:04.795031609+08:00 container start 0fdb...ff37 (image=alpine:latest, name=test)
2017-01-05T00:36:09.830268747+08:00 container kill 0fdb...ff37 (image=alpine:latest, name=test, signal=15)
2017-01-05T00:36:09.840186338+08:00 container die 0fdb...ff37 (exitCode=143, image=alpine:latest, name=test)
2017-01-05T00:36:09.880113663+08:00 network disconnect e2e...29e2 (container=0fdb...ff37, name=bridge, type=bridge)
2017-01-05T00:36:09.890214053+08:00 container stop 0fdb...ff37 (image=alpine:latest, name=test)

$ docker events --since '2013-09-03T15:49:29'
2017-01-05T00:35:41.241772953+08:00 volume create testVol (driver=local)
2017-01-05T00:35:58.859401177+08:00 container create d9cd...4d70 (image=alpine:latest, name=test)
2017-01-05T00:36:04.703631903+08:00 network connect e2e1...29e2 (container=0fdb...ff37, name=bridge, type=bridge)
2017-01-05T00:36:04.795031609+08:00 container start 0fdb...ff37 (image=alpine:latest, name=test)
2017-01-05T00:36:09.830268747+08:00 container kill 0fdb...ff37 (image=alpine:latest, name=test, signal=15)
2017-01-05T00:36:09.840186338+08:00 container die 0fdb...ff37 (exitCode=143, image=alpine:latest, name=test)
2017-01-05T00:36:09.880113663+08:00 network disconnect e2e...29e2 (container=0fdb...ff37, name=bridge, type=bridge)
2017-01-05T00:36:09.890214053+08:00 container stop 0fdb...ff37 (image=alpine:latest, name=test)

$ docker events --since '10m'
2017-01-05T00:35:41.241772953+08:00 volume create testVol (driver=local)
2017-01-05T00:35:58.859401177+08:00 container create d9cd...4d70 (image=alpine:latest, name=test)
2017-01-05T00:36:04.703631903+08:00 network connect e2e1...29e2 (container=0fdb...ff37, name=bridge, type=bridge)
2017-01-05T00:36:04.795031609+08:00 container start 0fdb...ff37 (image=alpine:latest, name=test)
2017-01-05T00:36:09.830268747+08:00 container kill 0fdb...ff37 (image=alpine:latest, name=test, signal=15)
2017-01-05T00:36:09.840186338+08:00 container die 0fdb...ff37 (exitCode=143, image=alpine:latest, name=test)
2017-01-05T00:36:09.880113663+08:00 network disconnect e2e...29e2 (container=0fdb...ff37, name=bridge, type=bridge)
2017-01-05T00:36:09.890214053+08:00 container stop 0fdb...ff37 (image=alpine:latest, name=test)
```

### Filter events by criteria

The following commands show several different ways to filter the `docker event`
output.

```bash
$ docker events --filter 'event=stop'

2017-01-05T00:40:22.880175420+08:00 container stop 0fdb...ff37 (image=alpine:latest, name=test)
2017-01-05T00:41:17.888104182+08:00 container stop 2a8f...4e78 (image=alpine, name=kickass_brattain)

$ docker events --filter 'image=alpine'

2017-01-05T00:41:55.784240236+08:00 container create d9cd...4d70 (image=alpine, name=happy_meitner)
2017-01-05T00:41:55.913156783+08:00 container start d9cd...4d70 (image=alpine, name=happy_meitner)
2017-01-05T00:42:01.106875249+08:00 container kill d9cd...4d70 (image=alpine, name=happy_meitner, signal=15)
2017-01-05T00:42:11.111934041+08:00 container kill d9cd...4d70 (image=alpine, name=happy_meitner, signal=9)
2017-01-05T00:42:11.119578204+08:00 container die d9cd...4d70 (exitCode=137, image=alpine, name=happy_meitner)
2017-01-05T00:42:11.173276611+08:00 container stop d9cd...4d70 (image=alpine, name=happy_meitner)

$ docker events --filter 'container=test'

2017-01-05T00:43:00.139719934+08:00 container start 0fdb...ff37 (image=alpine:latest, name=test)
2017-01-05T00:43:09.259951086+08:00 container kill 0fdb...ff37 (image=alpine:latest, name=test, signal=15)
2017-01-05T00:43:09.270102715+08:00 container die 0fdb...ff37 (exitCode=143, image=alpine:latest, name=test)
2017-01-05T00:43:09.312556440+08:00 container stop 0fdb...ff37 (image=alpine:latest, name=test)

$ docker events --filter 'container=test' --filter 'container=d9cdb1525ea8'

2017-01-05T00:44:11.517071981+08:00 container start 0fdb...ff37 (image=alpine:latest, name=test)
2017-01-05T00:44:17.685870901+08:00 container start d9cd...4d70 (image=alpine, name=happy_meitner)
2017-01-05T00:44:29.757658470+08:00 container kill 0fdb...ff37 (image=alpine:latest, name=test, signal=9)
2017-01-05T00:44:29.767718510+08:00 container die 0fdb...ff37 (exitCode=137, image=alpine:latest, name=test)
2017-01-05T00:44:29.815798344+08:00 container destroy 0fdb...ff37 (image=alpine:latest, name=test)

$ docker events --filter 'container=test' --filter 'event=stop'

2017-01-05T00:46:13.664099505+08:00 container stop a9d1...e130 (image=alpine, name=test)

$ docker events --filter 'type=volume'

2015-12-23T21:05:28.136212689Z volume create test-event-volume-local (driver=local)
2015-12-23T21:05:28.383462717Z volume mount test-event-volume-local (read/write=true, container=562f...5025, destination=/foo, driver=local, propagation=rprivate)
2015-12-23T21:05:28.650314265Z volume unmount test-event-volume-local (container=562f...5025, driver=local)
2015-12-23T21:05:28.716218405Z volume destroy test-event-volume-local (driver=local)

$ docker events --filter 'type=network'

2015-12-23T21:38:24.705709133Z network create 8b11...2c5b (name=test-event-network-local, type=bridge)
2015-12-23T21:38:25.119625123Z network connect 8b11...2c5b (name=test-event-network-local, container=b4be...c54e, type=bridge)

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
```

The `type=plugin` filter is experimental.

```bash
$ docker events --filter 'type=plugin'

2016-07-25T17:30:14.825557616Z plugin pull ec7b87f2ce84330fe076e666f17dfc049d2d7ae0b8190763de94e1f2d105993f (name=tiborvass/sample-volume-plugin:latest)
2016-07-25T17:30:14.888127370Z plugin enable ec7b87f2ce84330fe076e666f17dfc049d2d7ae0b8190763de94e1f2d105993f (name=tiborvass/sample-volume-plugin:latest)
```

### Format the output

```bash
{% raw %}
$ docker events --filter 'type=container' --format 'Type={{.Type}}  Status={{.Status}}  ID={{.ID}}'

Type=container  Status=create  ID=2ee349dac409e97974ce8d01b70d250b85e0ba8189299c126a87812311951e26
Type=container  Status=attach  ID=2ee349dac409e97974ce8d01b70d250b85e0ba8189299c126a87812311951e26
Type=container  Status=start  ID=2ee349dac409e97974ce8d01b70d250b85e0ba8189299c126a87812311951e26
Type=container  Status=resize  ID=2ee349dac409e97974ce8d01b70d250b85e0ba8189299c126a87812311951e26
Type=container  Status=die  ID=2ee349dac409e97974ce8d01b70d250b85e0ba8189299c126a87812311951e26
Type=container  Status=destroy  ID=2ee349dac409e97974ce8d01b70d250b85e0ba8189299c126a87812311951e26
{% endraw %}
```

#### Format as JSON

```none
{% raw %}
    $ docker events --format '{{json .}}'

    {"status":"create","id":"196016a57679bf42424484918746a9474cd905dd993c4d0f4..
    {"status":"attach","id":"196016a57679bf42424484918746a9474cd905dd993c4d0f4..
    {"Type":"network","Action":"connect","Actor":{"ID":"1b50a5bf755f6021dfa78e..
    {"status":"start","id":"196016a57679bf42424484918746a9474cd905dd993c4d0f42..
    {"status":"resize","id":"196016a57679bf42424484918746a9474cd905dd993c4d0f4..
{% endraw %}
```
