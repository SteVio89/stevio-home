#!/bin/sh
set -e

# Fix ownership of bind-mounted volumes so the stevio user can write to them.
# Host-mounted directories override the container's pre-set permissions.
chown -R stevio:stevio /data /assets /apps /home/stevio

exec gosu stevio "$@"
