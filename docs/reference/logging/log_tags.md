<!--[metadata]>
+++
title = "Log tags for logging driver"
description = "Describes how to format tags for."
keywords = ["docker, logging, driver, syslog, Fluentd, gelf"]
[menu.main]
parent = "smn_logging"
weight = 1
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

>**Note**:The driver specific log options `syslog-tag`, `fluentd-tag` and
>`gelf-tag` still work for backwards compatibility. However, going forward you
>should standardize on using the generic `tag` log option instead.
