#!/bin/sh
set -eu

: "${DATABASE_URL:?DATABASE_URL is required}"

BACKUP_DIR="${BACKUP_DIR:-./backups}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
BACKUP_FILE="${BACKUP_DIR}/otasign-${TIMESTAMP}.dump"

mkdir -p "$BACKUP_DIR"

pg_dump \
  --dbname="$DATABASE_URL" \
  --format=custom \
  --no-owner \
  --no-privileges \
  --file="$BACKUP_FILE"

find "$BACKUP_DIR" -type f -name 'otasign-*.dump' -mtime +"$RETENTION_DAYS" -delete

printf '%s\n' "$BACKUP_FILE"
