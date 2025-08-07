#!/bin/sh
set -eu

# Delegate to the api module's generation script
cd api && ./scripts/generate-swagger-api.sh
