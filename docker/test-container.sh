#!/usr/bin/env sh
make protobuf
ls -la ./proto/discord
echo "test" > /tmp/health.txt
tail -f /dev/null
