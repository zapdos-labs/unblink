#!/bin/sh
# Fix ownership of the data directory if it exists
if [ -d /data/unblink ]; then
  chown -R appuser:appuser /data/unblink
fi
# Drop to appuser and run the command
exec gosu appuser "$@"
