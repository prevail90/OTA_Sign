#!/bin/sh
set -eu

: "${DATABASE_URL:?DATABASE_URL is required}"

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 /path/to/otasign-backup.dump" >&2
  exit 2
fi

BACKUP_FILE="$1"

if [ ! -f "$BACKUP_FILE" ]; then
  echo "Backup file not found: $BACKUP_FILE" >&2
  exit 2
fi

pg_restore \
  --dbname="$DATABASE_URL" \
  --clean \
  --if-exists \
  --no-owner \
  --no-privileges \
  "$BACKUP_FILE"
