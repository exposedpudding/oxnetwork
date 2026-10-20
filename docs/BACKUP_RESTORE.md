# Backup and restore (Milestone 11, ADR-016)

The application commands and daily timer cover the **oxnet.work database** (inventory, host observations, workflows, runs, audit trail). They are not a whole-stack backup. The 20 October manual full-stack set also includes Authentik PostgreSQL and data, TLS certificates, Compose configuration, secrets and agent tokens (see [DEPLOYMENT_STATE.md](../deploy/home-gw/DEPLOYMENT_STATE.md)).

## What a backup contains

- A consistent snapshot of `oxnet.db`, taken while the app keeps running (SQLite `VACUUM INTO`, read-only on the source).
- **No session data.** `auth_sessions` (which holds CSRF tokens) and `oidc_login_flows` are emptied in the copy, and the bytes are rewritten, so a backup can never be used to hijack a session. After a restore everyone signs in again.
- The append-only audit triggers, all workflow definitions and run history, and the latest host and Tailscale observations. Observations are only as fresh as the backup; they go stale within minutes and refill on their own.
- A `.sha256` file beside it in `sha256sum` format. Backup files are owner-only (`0600`).
- It is **not encrypted**. Keep it on the same trust level as the host, or encrypt it when it leaves the machine (the owner's Windows copy uses DPAPI).

## Commands

The server binary has three maintenance subcommands (run them with `docker exec` or one-off containers; they never start the web server):

```text
oxnet-work backup  [-config FILE] [-db FILE] DEST          verified, session-free copy of the database
oxnet-work verify  BACKUP                                  integrity check, schema version, .sha256 comparison
oxnet-work restore [-config FILE] [-db FILE] [-force] BACKUP   replace the database (app must be stopped)
```

The database path comes from `-db`, else the config file, else `OXNET_DATA_DIR`. Output is JSON (path, bytes, SHA-256, schema version, integrity result, row counts for the main tables); the exit status is non-zero on any failure. `backup` refuses to overwrite a file. `verify` refuses a backup made by a **newer** version of oxnet.work (its schema is unknown to this build). `restore` needs `-force` to replace an existing database, always saves the current one first as `<db>.pre-restore-<UTC time>` (itself a verified backup), and refuses when the database has an active reader.

## On Home GW

```bash
cd /opt/oxnetwork/deploy/home-gw
sudo bash backup-oxnet.sh                 # backup into /opt/oxnetwork-backups/oxnet-db, keeps the newest 14
sudo OUT=/mnt/usb/oxnet KEEP=30 bash backup-oxnet.sh
sudo bash restore-oxnet.sh /opt/oxnetwork-backups/oxnet-db/oxnet-20261019T023000Z.db
```

`backup-oxnet.sh` snapshots inside the running container, copies the file out, re-verifies it, writes the checksum and prunes old copies. `restore-oxnet.sh` verifies the backup **before** stopping anything, then stops the app, restores into the data volume (keeping a safety copy there), hands file ownership back to the app user, starts the app and waits for `/readyz`. If the backup is rejected, the app is never stopped. If the restore itself fails, the old database is untouched and the app is restarted.

Schedule it on a new host (the timer is enabled on Home GW since 20 October 2026):

```bash
sudo cp oxnetwork-backup.service oxnetwork-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now oxnetwork-backup.timer
systemctl list-timers oxnetwork-backup.timer
```

**Copy backups off the host.** A backup on the same disk does not survive the disk. Keep at least one copy elsewhere and re-check it with `oxnet-work verify` (or `sha256sum -c`) after copying. The daily timer does not send data off the host. For a manually prepared full-stack tar, `deploy/home-gw/save-offhost-windows.ps1` streams it over SSH and saves only DPAPI CurrentUser encrypted bytes on Windows. Supply the Home GW SHA-256 with `-ExpectedSha256`; the script verifies the transfer and decrypts its output in memory to verify it again. The encrypted file depends on that Windows user's DPAPI profile. Maintain a separately recoverable encrypted copy and test its recovery.

## Undoing a restore

The previous database is in the data volume as `oxnet.db.pre-restore-<time>` (with its `.sha256`). To go back, stop the app and run `restore-oxnet.sh` with that file copied out (`docker cp`), or restore from the earlier backup you already had. A restore from an older backup also rewinds workflow runs recorded since; runs that were active are marked failed with `INTERRUPTED` when the app starts, and cron triggers missed while it was down are not replayed.

## Restore test procedure (repeat after every upgrade of the backup tooling, and at least quarterly)

Do this on a **different machine** from the one that produced the backup, using a scratch directory:

1. Copy one backup and its `.sha256` file to the test machine; run `sha256sum -c`.
2. `oxnet-work verify BACKUP` must report `integrity: ok` and a schema version no newer than this build.
3. `oxnet-work restore -db /scratch/restored/oxnet.db BACKUP`.
4. Start the app with `OXNET_DATA_DIR=/scratch/restored OXNET_LISTEN=127.0.0.1:18099` (demo mode is enough; the app migrates an older schema forward on start).
5. Check `/readyz`, then `/api/v1/workflows`, `/api/v1/workflow-runs` and `/api/v1/audit` return the data you expect, and that `auth_sessions` is empty.
6. Delete the scratch directory and any copy of the raw production file.

### Result of the test run on 18 October 2026

- **Source:** the live Home GW database backup `oxnet-post-reviewed-m4-2026-10-18.db` (155,648 bytes, schema 7, `integrity_check: ok`), copied host to host to OX-C-01.
- **Verified:** 8 inventory nodes, 6 host observations, 1 workflow, 7 audit events and 3 session rows. The clean backup had the 3 sessions removed and everything else identical.
- **Restored** into a fresh directory and started with the real server binary. The app migrated the database from schema 7 to 8, reported ready, and served the workflow `Check Home GW freshness` with its two successful runs (one cron, one manual by the owner) and the audit events.
- **Scripts:** `backup-oxnet.sh` and `restore-oxnet.sh` were run for real against a sandbox container: three backups with pruning to the newest two, a change made after the backup that the restore correctly removed, a safety copy kept, file ownership returned to the app user (uid 10001), a damaged backup refused before the app was stopped, and the app left running.
- **Then untested:** Authentik, certificates, Compose secrets and agent tokens. These were included in the separate-host drill on 20 October below. No production Home GW restore was performed.

### Full-stack drill on 20 October 2026

- Home GW produced a fresh PostgreSQL custom dump, Authentik data archive, private configuration archive, and verified session-free app SQLite backup. The combined root-only tar is `/opt/oxnetwork-backups/oxnetwork-offhost-20261020T1843Z.tar`; SHA-256 is `b5b153c71eb954845f0ad83c25e39606f166e4046678229aa32827948d491a07`. The manifest covers all five component files and passed after transfer to OX-C-01.
- On OX-C-01, `pg_restore --exit-on-error` loaded the dump into a fresh PostgreSQL 16 instance. Four Authentik users and two confirmed TOTP devices were present. The extracted TLS certificate was valid and its public key matched the private key. The restored app SQLite had `quick_check: ok`, migration 9, one workflow and eight inventory nodes.
- The restored PostgreSQL, Authentik 2026.8.4, Caddy and app started on a Docker network marked `internal`, with no host port bindings. Authentik live and ready endpoints returned HTTP 200. Through restored trusted HTTPS, the app returned `/readyz` 200 and unauthenticated `/api/v1/system` 401; both configured service probes were healthy. The scratch stack and all secret-bearing scratch files were removed afterward.
- A Windows off-host copy at `C:\Users\Lee\Documents\Codex\2026-10-18\ru\outputs\oxnetwork-offhost-20261020T1843Z.tar.dpapi` passed DPAPI decrypt-and-hash verification. This drill used the transferred plaintext tar, not a recovery from the DPAPI copy. It did not exercise an owner OIDC login/MFA, production DNS/ACME/Tailscale Service binding, or restoration on Home GW itself. A second encrypted copy with recovery independent of this Windows profile remains open.

## Limits and notes

- `backup` needs free disk space about the size of the database (a few hundred KB today).
- A running app is never paused: a backup reflects one instant, and writes that happen during it appear in the next backup.
- `verify` against a live WAL-mode database file creates `-wal` and `-shm` files beside it; run it on backup files, not on `oxnet.db`.
- The restore command cannot see an idle connection from a running app, which is why the script stops the container first. Do not run `restore` by hand against a database a running app has open.
