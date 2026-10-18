#!/usr/bin/env bash
# Non-authenticated transport checks on Home GW after Caddy starts.
set -euo pipefail
cd "$(dirname "$0")"

[[ -f certs/fullchain.pem && -f certs/key.pem ]] || { echo 'Certificate files are missing.' >&2; exit 1; }
openssl x509 -in certs/fullchain.pem -noout -checkend 86400 >/dev/null
for name in auth.oxnet.work oxnet.work; do
  printf '%s certificate and TLS: ' "$name"
  curl --noproxy '*' --silent --show-error --output /dev/null --max-time 10 \
    --connect-to "$name:443:127.0.0.1:8445" \
    --write-out '%{http_code}\n' "https://$name/"
done
printf 'Authenticated OIDC flow, private DNS, Service reachability and internet isolation still require client-side checks.\n'
