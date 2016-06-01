<!--[metadata]>
+++
aliases = ["/engine/reference/logging/log_tags/"]
title = "Log tags for logging driver"
description = "Describes how to format tags for."
keywords = ["docker, logging, driver, syslog, Fluentd, gelf, journald"]
[menu.main]
parent = "smn_logging"
weight = -1
+++
<![end-metadata]-->

# Log Tags

The `tag` log option specifies how to format a tag that identifies the
container's log messages. By default, the system uses the first 12 characters of
the container id. To override this behavior, specify a `tag` option:

```
docker run --log-driver=fluentd --log-opt fluentd-address=myhost.local:24224 --log-opt tag="mailer"
```

Docker supports some special template markup you can use when specifying a tag's value:

| Markup             | Description                                          |
|--------------------|------------------------------------------------------|
| `{{.ID}}`          | The first 12 characters of the container id.         |
| `{{.FullID}}`      | The full container id.                               |
| `{{.Name}}`        | The container name.                                  |
| `{{.ImageID}}`     | The first 12 characters of the container's image id. |
| `{{.ImageFullID}}` | The container's full image identifier.               |
| `{{.ImageName}}`   | The name of the image used by the container.         |
| `{{.DaemonName}}`  | The name of the docker program (`docker`).           |

For example, specifying a `--log-opt tag="{{.ImageName}}/{{.Name}}/{{.ID}}"` value yields `syslog` log lines like:

```
Aug  7 18:33:19 HOSTNAME docker/hello-world/foobar/5790672ab6a0[9103]: Hello from Docker.
```

At startup time, the system sets the `container_name` field and `{{.Name}}` in
the tags. If you use `docker rename` to rename a container, the new name is not
reflected in the log messages. Instead, these messages continue to use the
original container name.

For advanced usage, the generated tag's use [go
templates](http://golang.org/pkg/text/template/) and the container's [logging
context](https://github.com/docker/docker/blob/master/daemon/logger/context.go).

As an example of what is possible with the syslog logger:

```
$ docker run -it --rm \
    --log-driver syslog \
    --log-opt tag="{{ (.ExtraAttributes nil).SOME_ENV_VAR }}" \
    --log-opt env=SOME_ENV_VAR \
    -e SOME_ENV_VAR=logtester.1234 \
    flyinprogrammer/logtester
```

Results in logs like this:

```
Apr  1 15:22:17 ip-10-27-39-73 docker/logtester.1234[45499]: + exec app
Apr  1 15:22:17 ip-10-27-39-73 docker/logtester.1234[45499]: 2016-04-01 15:22:17.075416751 +0000 UTC stderr msg: 1
```

