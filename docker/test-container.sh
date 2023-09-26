#!/usr/bin/env sh
echo "test" > /tmp/health.txt
(cd proto && make discord_proto && make health_proto)
tail -f /dev/null
