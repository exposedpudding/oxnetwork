#!/usr/bin/env bash
# Back up the oxnet.work SQLite database from the running container (ADR-016).
#
#   sudo bash backup-oxnet.sh                 # writes a verified backup, keeps the newest 14
#   sudo OUT=/mnt/usb/oxnet KEEP=30 bash backup-oxnet.sh
#
# The copy is a consistent snapshot taken while the app keeps running. Session and
# CSRF tokens are removed from it. It is integrity-checked inside the container and
# again after being copied out, and a .sha256 file is written beside it.
# This covers the oxnet.work database only. Authentik's PostgreSQL, the certificates,
# the Compose .env and the secrets directory need their own backups (DEPLOYMENT_STATE.md).
set -euo pipefail

OUT="${OUT:-/opt/oxnetwork-backups/oxnet-db}"
KEEP="${KEEP:-14}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
NAME="oxnet-${STAMP}.db"

cid="$(docker ps -q --filter label=com.docker.compose.service=oxnet-work | head -n1)"
if [ -z "$cid" ]; then
  echo "oxnet-work is not running; nothing to back up from. To copy a stopped volume, see docs/BACKUP_RESTORE.md." >&2
  exit 1
fi
image="$(docker inspect -f '{{.Config.Image}}' "$cid")"

umask 077
mkdir -p "$OUT"
chmod 700 "$OUT"

echo "creating $NAME inside the container"
docker exec "$cid" /app/oxnet-work backup "/data/backups/${NAME}" >/dev/null
docker cp "${cid}:/data/backups/${NAME}" "${OUT}/${NAME}"
docker cp "${cid}:/data/backups/${NAME}.sha256" "${OUT}/${NAME}.sha256"
docker exec "$cid" rm -f "/data/backups/${NAME}" "/data/backups/${NAME}.sha256"
chmod 600 "${OUT}/${NAME}" "${OUT}/${NAME}.sha256"

echo "verifying the copied file"
( cd "$OUT" && sha256sum -c "${NAME}.sha256" )
# Root and no network: the backup directory is root-only and verification needs nothing else.
docker run --rm --user 0 --network none --entrypoint /app/oxnet-work -v "${OUT}:/b:ro" "$image" verify "/b/${NAME}" | grep -E '"(integrity|migration_version|bytes)"'

# Keep the newest $KEEP backups.
ls -1t "${OUT}"/oxnet-*.db 2>/dev/null | tail -n +"$((KEEP + 1))" | while read -r old; do
  rm -f "$old" "${old}.sha256"
  echo "pruned $(basename "$old")"
done
echo "backup complete: ${OUT}/${NAME}"
echo "Copy it off this machine (and test a restore) as described in docs/BACKUP_RESTORE.md."
