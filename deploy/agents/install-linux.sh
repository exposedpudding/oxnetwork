#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 3 || ! $1 =~ ^tailscale-[0-9]+$ ]]; then
  printf 'Usage: sudo %s tailscale-NODEID /path/to/oxnet-host-agent-linux-amd64 /path/to/node-token\n' "$0" >&2
  exit 2
fi
if [[ $(id -u) -ne 0 ]]; then
  printf 'Run as root.\n' >&2
  exit 2
fi

node_id=$1
binary=$2
token=$3
install -d -m 755 /usr/local/libexec
install -d -m 700 /etc/oxnetwork/host-agents
install -m 755 "$binary" /usr/local/libexec/oxnet-host-agent
install -m 600 "$token" "/etc/oxnetwork/host-agents/$node_id"
install -m 644 "$(dirname "$0")/oxnet-host-agent@.service" /etc/systemd/system/oxnet-host-agent@.service
systemctl daemon-reload
systemctl enable --now "oxnet-host-agent@$node_id.service"
systemctl restart "oxnet-host-agent@$node_id.service"
printf 'Installed oxnet-host-agent for %s.\n' "$node_id"
