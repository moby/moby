#!/bin/sh
set -eu

swagger generate model -f api/swagger.yaml \
    -t api -m types --skip-validator \
    -n Volume \
    -n Port \
    -n ImageSummary
