# OTA Sign Operations Runbook

## Production Services

OTA Sign production has these critical dependencies:

- OTA Sign backend container
- OTA Sign frontend container
- PostgreSQL database
- Moodle OTA Sign Connector
- DocuSeal API and webhooks
- Reverse proxy / TLS entrypoint
- Optional notification webhook target

## Deployment Checklist

1. Push a committed branch to GitHub.
2. Confirm CI passes for backend, frontend, and Moodle plugin checks.
3. Confirm GHCR images exist for the intended `OTASIGN_IMAGE_TAG`.
4. Set production stack variables in Portainer or the deployment environment:
   - `OTASIGN_IMAGE_TAG`
   - `OTASIGN_FRONTEND_URL`
   - `MOODLE_LOGIN_URL`
   - `MOODLE_OTA_SIGN_LAUNCH_URL`
   - `MOODLE_LAUNCH_SIGNING_SECRET`
   - `DATABASE_URL`
   - `DOCUSEAL_URL`
   - `DOCUSEAL_PUBLIC_URL`
   - `DOCUSEAL_API_KEY`
   - `DOCUSEAL_WEBHOOK_SECRET`
   - `NOTIFICATION_WEBHOOK_URL` if used
   - `NOTIFICATION_WEBHOOK_SECRET` if used
5. Deploy `docker-compose.prod.example.yml` as the production stack.
6. Confirm `/readyz` returns `200`.
7. Confirm `/healthz/full` reports database and DocuSeal status.
8. Launch from Moodle with a test account.
9. Create a DocuSeal submission, complete it, receive the webhook, and download the completed PDF through OTA Sign.

## Database Migrations

Backend startup applies SQL files from `DATABASE_MIGRATIONS_PATH`, defaulting to `db/migrations`.

Rules:

- Add new migrations as timestamped or numbered `.sql` files.
- Never edit an already-applied production migration.
- The backend records applied files in `schema_migrations`.
- Migrations run before the API starts serving requests.

Emergency check:

```bash
psql "$DATABASE_URL" -c "select version, applied_at from schema_migrations order by version;"
```

## Backups

Run logical PostgreSQL backups with:

```bash
DATABASE_URL='postgres://...' BACKUP_DIR=/secure/backups/otasign ./scripts/backup-postgres.sh
```

Recommended schedule:

- Hourly backups during active pilot periods.
- Daily backups after the system stabilizes.
- Keep at least 14 days locally.
- Copy backups to a separate encrypted location.

Validate backup freshness:

```bash
find /secure/backups/otasign -name 'otasign-*.dump' -mtime -1 -print
```

## Restore

Restore into a new or intentionally cleared database:

```bash
DATABASE_URL='postgres://...' ./scripts/restore-postgres.sh /secure/backups/otasign/otasign-YYYYMMDDTHHMMSSZ.dump
```

Recovery drill:

1. Restore the latest backup into a non-production database.
2. Start the backend against the restored database.
3. Confirm `/readyz`.
4. Confirm users, templates, submissions, signer state, and webhook event records are present.

## Log Review

Review backend logs for:

- `moodle launch rejected`
- `docuseal template sync failed`
- `docuseal create submission failed`
- `docuseal webhook rejected`
- `process docuseal webhook event failed`
- `send commander signature notification failed`

Container log command:

```bash
docker logs --since 1h otasign-backend
```

If Portainer owns the stack, use the container log view plus the same search terms above.

## Monitoring And Alerts

Minimum alert checks:

- Backend `/readyz` fails.
- Backend `/healthz/full` fails or reports DocuSeal error.
- Frontend container health check fails.
- PostgreSQL backup older than the expected schedule.
- Repeated webhook rejection or submission creation errors in logs.
- Disk usage above 80 percent on the Docker host or database host.

Simple external health probe:

```bash
OTASIGN_BACKEND_URL=https://sign.example.com ./scripts/healthcheck.sh
```

## Secret Rotation

Rotate secrets one at a time and verify the system between each rotation.

Moodle launch signing secret:

1. Generate a new high-entropy secret.
2. Update `MOODLE_LAUNCH_SIGNING_SECRET` in the backend stack.
3. Update the same secret in the Moodle OTA Sign Connector settings.
4. Redeploy backend.
5. Launch from Moodle and confirm a session starts.

DocuSeal API key:

1. Create a new DocuSeal API key.
2. Update `DOCUSEAL_API_KEY`.
3. Redeploy backend.
4. Confirm `/healthz/full`.
5. Revoke the old key.

DocuSeal webhook secret:

1. Set the new secret in DocuSeal webhook settings.
2. Update `DOCUSEAL_WEBHOOK_SECRET`.
3. Redeploy backend.
4. Complete a test submission and confirm the webhook is accepted.

Notification webhook secret:

1. Update the receiver secret.
2. Update `NOTIFICATION_WEBHOOK_SECRET`.
3. Send a test notification payload.

## Incident Recovery

Backend unhealthy:

1. Check `/readyz` and `/healthz/full`.
2. Check recent backend logs.
3. Confirm database connectivity.
4. Confirm DocuSeal URL and API key.
5. Roll back to the previous image tag if the issue began after deploy.

Bad deployment:

1. Set `OTASIGN_IMAGE_TAG` to the last known good image tag.
2. Redeploy the stack.
3. Confirm health endpoints.
4. Run a Moodle launch smoke test.

Database loss:

1. Stop backend writes by stopping the backend container.
2. Restore the latest verified backup into the database.
3. Start the backend.
4. Confirm migrations, health endpoints, and core records.
5. Run a DocuSeal status refresh by opening affected dashboards.

Webhook outage:

1. Confirm DocuSeal webhook URL and secret.
2. Check `docuseal_webhook_events`.
3. Use `/healthz/full` to verify DocuSeal API access.
4. Open affected dashboards to refresh pending submission status.
