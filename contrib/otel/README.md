# Sample stack for testing OTEL functionality with moby

To easily test the OTEL functionality present in moby, you can spin up a small demo compose stack that includes:
- an OTEL collector container;
- a Jaeger container to visualize traces;
- an alternative Aspire Dashboard container to visualize traces;

The OTEL collector is configured to export Traces to both the Jaeger and Aspire containers.

The `contrib/otel` directory contains the compose file with the services configured, along with a basic configuration file for the OTEL collector.

## How can I use it?

1. Export the env var used to override the OTLP endpoint:  
  `export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318` (if running in a devcontainer or in other ways, you might have to change how you pass this env var to the daemon);
2. Start the moby engine you want to get traces from (make sure it gets the env var declared above);
3. Start the otel compose stack by running `docker compose up -d` in the `contrib/otel/` directory;
4. Make some calls to the engine using the Docker CLI to send some traces to the endpoint;
5. Browse Jaeger at `http://localhost:16686` or the Aspire Dashboard at `http://localhost:18888/traces`;
6. To see some traces from the engine, select `dockerd` in the top left dropdown

> **Note**: The precise steps may vary based on how you're working on the codebase (buiding a binary and executing natively, running/debugging in a devcontainer, etc... )

## Cleanup?

Simply run `docker compose down` in the `contrib/otel/` directory.

You can also run `unset OTEL_EXPORTER_OTLP_ENDPOINT` to get rid of the OTLP env var from your environment
