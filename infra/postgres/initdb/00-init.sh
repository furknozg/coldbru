#!/usr/bin/env bash
# Runs once, on an empty data volume, via the postgres image entrypoint.
# Files in ./sql are not picked up by the entrypoint; this script fixes their order.
set -Eeuo pipefail

: "${CUSTOMER_DB_PASSWORD:?CUSTOMER_DB_PASSWORD must be set}"
SQL_DIR="/docker-entrypoint-initdb.d/sql"

run_sql() {
  local db="$1" file="$2"; shift 2
  echo "initdb: ${file} on ${db}"
  psql -v ON_ERROR_STOP=1 --no-psqlrc --username "$POSTGRES_USER" --no-password --dbname "$db" "$@" -f "${SQL_DIR}/${file}"
}

run_sql "$POSTGRES_DB" 01_customer_db.sql -v customer_db_password="$CUSTOMER_DB_PASSWORD"
run_sql customer_db   02_customer_schema.sql
run_sql customer_db   03_customer_seed.sql
