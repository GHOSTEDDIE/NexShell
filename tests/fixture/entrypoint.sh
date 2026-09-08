#!/bin/sh
set -eu
: "${TEST_PASSWORD:?TEST_PASSWORD is required}"
printf 'root:%s\n' "$TEST_PASSWORD" | chpasswd
printf 'healthy baseline\n' > /tmp/fixture/healthy.txt
exec /usr/sbin/sshd -D -e
