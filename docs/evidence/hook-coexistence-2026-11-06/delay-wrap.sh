#!/usr/bin/env bash
# Timing probe only: sleep <secs>, then run the wrapped hook with the same stdin/stdout.
d=$1; shift; sleep "$d"; exec "$@"
