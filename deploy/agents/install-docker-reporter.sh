#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 3 || ! $1 =~ ^tailscale-[0-9]+$ ]]; then
  printf 'Usage: sudo %s tailscale-NODEID /path/to/oxnet-docker-reporter-linux-amd64 /path/to/node-token\n' "$0" >&2
  exit 2
fi
if [[ $(id -u) -ne 0 ]]; then
  printf 'Run as root.\n' >&2
  exit 2
fi
if ! getent group docker >/dev/null; then
  printf 'This host has no docker group; is Docker installed?\n' >&2
  exit 2
fi

node_id=$1
binary=$2
token=$3
install -d -m 755 /usr/local/libexec
install -d -m 700 /etc/oxnetwork/docker-reporters
install -m 755 "$binary" /usr/local/libexec/oxnet-docker-reporter
install -m 600 "$token" "/etc/oxnetwork/docker-reporters/$node_id"
install -m 644 "$(dirname "$0")/oxnet-docker-reporter@.service" /etc/systemd/system/oxnet-docker-reporter@.service
systemctl daemon-reload
systemctl enable --now "oxnet-docker-reporter@$node_id.service"
systemctl restart "oxnet-docker-reporter@$node_id.service"
printf 'Installed oxnet-docker-reporter for %s.\n' "$node_id"