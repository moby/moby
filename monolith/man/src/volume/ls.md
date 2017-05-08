Lists all the volumes Docker manages. You can filter using the `-f` or
`--filter` flag. The filtering format is a `key=value` pair. To specify
more than one filter,  pass multiple flags (for example,
`--filter "foo=bar" --filter "bif=baz"`)

The currently supported filters are:

* `dangling` (boolean - `true` or `false`, `1` or `0`)
* `driver` (a volume driver's name)
* `label` (`label=<key>` or `label=<key>=<value>`)
* `name` (a volume's name)
