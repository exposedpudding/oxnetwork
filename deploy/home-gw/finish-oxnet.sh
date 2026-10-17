#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
umask 077

[[ -f .env ]] || { echo 'Run bootstrap-authentik.sh first.' >&2; exit 1; }
[[ -r /dev/tty ]] || { echo 'An interactive terminal is required to enter the OIDC credentials.' >&2; exit 1; }
grep -q '^OXNET_OIDC_CLIENT_ID=' .env || { echo 'Missing OXNET_OIDC_CLIENT_ID entry in .env.' >&2; exit 1; }
command -v curl >/dev/null || { echo 'Missing required tool: curl' >&2; exit 1; }

IFS= read -r -p 'Authentik OIDC client ID: ' client_id < /dev/tty
IFS= read -r -s -p 'Authentik OIDC client secret: ' client_secret < /dev/tty
printf '\n' > /dev/tty
[[ "$client_id" =~ ^[A-Za-z0-9._-]+$ && -n "$client_secret" ]] || {
  echo 'Client ID or secret is empty/invalid.' >&2
  exit 1
}

mkdir -p secrets
chmod 700 secrets
printf '%s' "$client_secret" > secrets/oidc_client_secret
unset client_secret
if [[ "$(id -u)" == 0 ]]; then
  chown root:10001 secrets/oidc_client_secret
  chmod 640 secrets/oidc_client_secret
else
  sudo chown root:10001 secrets/oidc_client_secret
  sudo chmod 640 secrets/oidc_client_secret
fi

temp_env="$(mktemp .env.XXXXXX)"
trap 'rm -f "$temp_env"' EXIT
while IFS= read -r line || [[ -n "$line" ]]; do
  if [[ "$line" == OXNET_OIDC_CLIENT_ID=* ]]; then
    printf 'OXNET_OIDC_CLIENT_ID=%s\n' "$client_id" >> "$temp_env"
  else
    printf '%s\n' "$line" >> "$temp_env"
  fi
done < .env
chmod 600 "$temp_env"
mv "$temp_env" .env
trap - EXIT
unset client_id

docker compose --env-file .env -f compose.yml config --quiet
docker compose --env-file .env -f compose.yml build oxnet-work
docker compose --env-file .env -f compose.yml run --rm --no-deps --entrypoint /bin/sh oxnet-work -c 'test -r /run/secrets/oidc_client_secret'
docker compose --env-file .env -f compose.yml up -d oxnet-work
curl --fail --silent --show-error --max-time 10 http://127.0.0.1:8080/readyz
echo
echo 'oxnet.work started. Complete the private HTTPS, login and internet-inaccessibility checks in README.md.'
