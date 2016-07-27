<!--[metadata]>
+++
title = "swarm join-token"
description = "The swarm join-token command description and usage"
keywords = ["swarm, join-token"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# swarm join-token

```markdown
Usage:	docker swarm join-token [--rotate] (worker|manager)

Manage join tokens

Options:
      --help     Print usage
  -q, --quiet    Only display token
      --rotate   Rotate join token
```

Join tokens are secrets that determine whether or not a node will join the swarm as a manager node
or a worker node. You pass the token using the `--token flag` when you run
[swarm join](swarm_join.md). You can access the current tokens or rotate the tokens using
`swarm join-token`.

Run with only a single `worker` or `manager` argument, it will print a command for joining a new
node to the swarm, including the necessary token:

```bash
$ docker swarm join-token worker
To add a worker to this swarm, run the following command:
    docker swarm join \
    --token SWMTKN-1-3pu6hszjas19xyp7ghgosyx9k8atbfcr8p2is99znpy26u2lkl-1awxwuwd3z9j1z3puu7rcgdbx \
    172.17.0.2:2377

$ docker swarm join-token manager
To add a manager to this swarm, run the following command:
    docker swarm join \
    --token SWMTKN-1-3pu6hszjas19xyp7ghgosyx9k8atbfcr8p2is99znpy26u2lkl-7p73s1dx5in4tatdymyhg9hu2 \
    172.17.0.2:2377
```

Use the `--rotate` flag to generate a new join token for the specified role:

```bash
$ docker swarm join-token --rotate worker
To add a worker to this swarm, run the following command:
    docker swarm join \
    --token SWMTKN-1-3pu6hszjas19xyp7ghgosyx9k8atbfcr8p2is99znpy26u2lkl-b30ljddcqhef9b9v4rs7mel7t \
    172.17.0.2:2377
```

After using `--rotate`, only the new token will be valid for joining with the specified role.

The `-q` (or `--quiet`) flag only prints the token:

```bash
$ docker swarm join-token -q worker
SWMTKN-1-3pu6hszjas19xyp7ghgosyx9k8atbfcr8p2is99znpy26u2lkl-b30ljddcqhef9b9v4rs7mel7t
```

### `--rotate`

Update the join token for a specified role with a new token and print the token.

### `--quiet`

Only print the token. Do not print a complete command for joining.

## Related information

* [swarm join](swarm_join.md)
