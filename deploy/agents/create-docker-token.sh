#!/usr/bin/env bash
# Run on Home GW, once for each Tailscale inventory node that will report containers.
# The token is separate from the host metrics token. No token is printed.
set -euo pipefail
if [[ $# -ne 1 || ! $1 =~ ^tailscale-[0-9]+$ ]]; then
  printf 'Usage: %s tailscale-NODEID\n' "$0" >&2
  exit 2
fi
if [[ $(id -u) -ne 0 ]]; then
  printf 'Run as root.\n' >&2
  exit 2
fi
node_id=$1
directory=/opt/oxnetwork/deploy/home-gw/secrets/docker-agents
mkdir -p "$directory"
chown 0:10001 "$directory"
chmod 750 "$directory"
python3 - "$directory/$node_id" <<'PY'
import os
import secrets
import sys

path = sys.argv[1]
fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o640)
try:
    os.write(fd, (secrets.token_urlsafe(48) + "\n").encode())
finally:
    os.close(fd)
os.chown(path, 0, 10001)
PY
printf 'Created a protected per-node container-report token for %s.\n' "$node_id"