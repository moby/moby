<!--[metadata]>
+++
title = "inspect"
description = "The inspect command description and usage"
keywords = ["inspect, container, json"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# inspect

    Usage: docker inspect [OPTIONS] CONTAINER|IMAGE [CONTAINER|IMAGE...]

    Return low-level information on a container or image

      -f, --format=""         Format the output using the given go template
      --help                  Print usage
      --type=container|image  Return JSON for specified type, permissible
                              values are "image" or "container"
      -s, --size              Display total file sizes if the type is container

By default, this will render all results in a JSON array. If the container and
image have the same name, this will return container JSON for unspecified type.
If a format is specified, the given template will be executed for each result.

Go's [text/template](http://golang.org/pkg/text/template/) package
describes all the details of the format.

## Examples

**Get an instance's IP address:**

For the most part, you can pick out any field from the JSON in a fairly
straightforward manner.

    $ docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $INSTANCE_ID

**Get an instance's MAC Address:**

For the most part, you can pick out any field from the JSON in a fairly
straightforward manner.

    $ docker inspect --format='{{range .NetworkSettings.Networks}}{{.MacAddress}}{{end}}' $INSTANCE_ID

**Get an instance's log path:**

    $ docker inspect --format='{{.LogPath}}' $INSTANCE_ID

**List All Port Bindings:**

One can loop over arrays and maps in the results to produce simple text
output:

    $ docker inspect --format='{{range $p, $conf := .NetworkSettings.Ports}} {{$p}} -> {{(index $conf 0).HostPort}} {{end}}' $INSTANCE_ID

**Find a Specific Port Mapping:**

The `.Field` syntax doesn't work when the field name begins with a
number, but the template language's `index` function does. The
`.NetworkSettings.Ports` section contains a map of the internal port
mappings to a list of external address/port objects. To grab just the
numeric public port, you use `index` to find the specific port map, and
then `index` 0 contains the first object inside of that. Then we ask for
the `HostPort` field to get the public address.

    $ docker inspect --format='{{(index (index .NetworkSettings.Ports "8787/tcp") 0).HostPort}}' $INSTANCE_ID

**Get a subsection in JSON format:**

If you request a field which is itself a structure containing other
fields, by default you get a Go-style dump of the inner values.
Docker adds a template function, `json`, which can be applied to get
results in JSON format.

    $ docker inspect --format='{{json .Config}}' $INSTANCE_ID
