#!/bin/sh

mkdocs serve &
echo "Waiting for 5 seconds to allow mkdocs server to be ready"
sleep 5
./docvalidate.py
