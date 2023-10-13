#!/usr/bin/env sh
make protobuf
echo "test" > /tmp/health.txt
tail -f /dev/null
