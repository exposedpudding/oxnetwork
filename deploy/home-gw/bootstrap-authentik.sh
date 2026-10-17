#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
umask 077

for tool in docker openssl; do
  command -v "$tool" >/dev/null || { echo "Missing required tool: $tool" >&2; exit 1; }
done
docker compose version >/dev/null

if [[ ! -f .env ]]; then
  temp_env="$(mktemp .env.XXXXXX)"
  trap 'rm -f "$temp_env"' EXIT
  printf 'PG_PASS=%s\n' "$(openssl rand -base64 36 | tr -d '\n')" > "$temp_env"
  printf 'AUTHENTIK_SECRET_KEY=%s\n' "$(openssl rand -base64 60 | tr -d '\n')" >> "$temp_env"
  printf 'OXNET_OIDC_ISSUER_URL=https://auth.oxnet.work/application/o/oxnet-work/\n' >> "$temp_env"
  printf 'OXNET_OIDC_CLIENT_ID=pending\n' >> "$temp_env"
  chmod 600 "$temp_env"
  mv "$temp_env" .env
  trap - EXIT
  echo 'Created .env with new local secrets. Keep it in an encrypted backup.'
else
  echo 'Using existing .env; no credentials were replaced.'
fi

mkdir -p secrets
chmod 700 secrets
docker compose --env-file .env -f compose.yml config --quiet
docker compose --env-file .env -f compose.yml up -d postgresql authentik-server authentik-worker
docker compose --env-file .env -f compose.yml ps
echo 'Authentik containers started. Configure private HTTPS and the OIDC application before running finish-oxnet.sh.'
