#!/usr/bin/env bash
# Read-only status snapshot for the Home GW agent. Does not display secret files.
set -u
cd "$(dirname "$0")" || exit 1

section() { printf '\n== %s ==\n' "$1"; }
section 'Source and local changes'
git -C ../.. log -1 --format='%h %s' 2>&1 || true
git -C ../.. status --short 2>&1 || true
if [[ -f LOCAL_CHANGES.md ]]; then printf 'LOCAL_CHANGES.md exists; review before pull\n'; fi

section 'Host capacity'
uname -sm
free -h 2>&1 || true
df -h . 2>&1 || true

section 'Tailscale'
tailscale version 2>&1 || true
tailscale serve status 2>&1 || true
tailscale service list --json 2>&1 || true

section 'Listeners relevant to this stack'
ss -ltnu 2>&1 | awk 'NR==1 || $5 ~ /:(53|443|8443|8445|8080|8081|8090|9000)$/' || true

section 'Compose services'
if [[ -f .env ]]; then
  docker compose --env-file .env -f compose.yml ps 2>&1 || true
else
  printf '.env is absent\n'
fi

section 'Local HTTP status codes'
for endpoint in http://127.0.0.1:9000/ http://127.0.0.1:8090/readyz; do
  curl --silent --output /dev/null --max-time 5 --write-out "$endpoint -> %{http_code}\n" "$endpoint" 2>&1 || true
done

section 'Certificate presence and expiry'
if [[ -f certs/fullchain.pem ]]; then
  openssl x509 -in certs/fullchain.pem -noout -dates -ext subjectAltName 2>&1 || true
else
  printf 'certs/fullchain.pem is absent\n'
fi
