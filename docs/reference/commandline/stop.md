---
title: "stop"
description: "The stop command description and usage"
keywords: ["stop, SIGKILL, SIGTERM"]
---

# stop

```markdown
Usage:  docker stop [OPTIONS] CONTAINER [CONTAINER...]

Stop one or more running containers

Options:
      --help       Print usage
  -t, --time int   Seconds to wait for stop before killing it (default 10)
```

The main process inside the container will receive `SIGTERM`, and after a grace
period, `SIGKILL`.
