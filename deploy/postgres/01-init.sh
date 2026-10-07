#!/bin/sh
set -eu
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
  --set=app_password="$APP_DB_PASSWORD" --set=keycloak_password="$KEYCLOAK_DB_PASSWORD" <<'SQL'
CREATE ROLE saveboard LOGIN PASSWORD :'app_password';
CREATE DATABASE saveboard OWNER saveboard;
CREATE ROLE keycloak LOGIN PASSWORD :'keycloak_password';
CREATE DATABASE keycloak OWNER keycloak;
REVOKE ALL ON DATABASE saveboard FROM PUBLIC;
REVOKE ALL ON DATABASE keycloak FROM PUBLIC;
SQL
