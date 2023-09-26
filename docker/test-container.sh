#!/usr/bin/env sh
(cd proto && make discord_proto && make health_proto)
tail -f /dev/null
