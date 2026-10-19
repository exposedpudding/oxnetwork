#!/usr/bin/env bash
# Replace the oxnet.work database with a verified backup (ADR-016).
#
#   sudo bash restore-oxnet.sh /opt/oxnetwork-backups/oxnet-db/oxnet-20261019T020000Z.db
#
# The app is stopped for the restore. The current database is kept first as a safety
# copy inside the data volume (oxnet.db.pre-restore-<time>). Everyone is signed out
# afterwards because backups never contain session data. The schema is brought up to
# date by the app on start; a backup from a newer version is refused.
set -euo pipefail

ready_url="${READY_URL:-http://127.0.0.1:8090/readyz}"   # Home GW publishes the app on loopback 8090
backup="${1:-}"
if [ -z "$backup" ] || [ ! -f "$backup" ]; then
  echo "usage: sudo bash restore-oxnet.sh /path/to/oxnet-YYYYMMDDTHHMMSSZ.db" >&2
  exit 2
fi
backup="$(readlink -f "$backup")"
dir="$(dirname "$backup")"
file="$(basename "$backup")"

cid="$(docker ps -aq --filter label=com.docker.compose.service=oxnet-work | head -n1)"
[ -n "$cid" ] || { echo "no oxnet-work container found" >&2; exit 1; }
image="$(docker inspect -f '{{.Config.Image}}' "$cid")"
volume="$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Name}}{{end}}{{end}}' "$cid")"
[ -n "$volume" ] || { echo "could not find the /data volume" >&2; exit 1; }

echo "1/4 verifying $file"
docker run --rm --user 0 --network none --entrypoint /app/oxnet-work -v "${dir}:/b:ro" "$image" verify "/b/${file}" | grep -E '"(integrity|migration_version|sha256)"'

echo "2/4 stopping oxnet-work"
docker stop "$cid" >/dev/null

echo "3/4 restoring into volume ${volume}"
# Root can read the root-only backup directory; ownership of everything written to
# the volume is handed back to the app user (uid 10001) straight afterwards.
if ! docker run --rm --user 0 --network none --entrypoint /app/oxnet-work -v "${volume}:/data" -v "${dir}:/b:ro" "$image" \
      restore -force -db /data/oxnet.db "/b/${file}"; then
  echo "restore failed; the existing database was not replaced. Starting the app again." >&2
  docker start "$cid" >/dev/null
  exit 1
fi
docker run --rm --user 0 --network none --entrypoint sh -v "${volume}:/data" "$image" -c 'chown -R 10001:10001 /data'

echo "4/4 starting oxnet-work"
docker start "$cid" >/dev/null
for _ in $(seq 1 30); do
  if [ "$(curl -s -o /dev/null -w '%{http_code}' "$ready_url")" = "200" ]; then
    echo "oxnet.work is ready on the restored database."
    echo "Safety copy of the previous database: /data/oxnet.db.pre-restore-* in volume ${volume}."
    exit 0
  fi
  sleep 2
done
echo "the app did not become ready; check 'docker logs ${cid}'. The safety copy is in the data volume." >&2
exit 1
