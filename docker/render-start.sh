#!/bin/sh
# Render entrypoint: maps Render DATABASE_URL / REDIS_URL / PORT into a runtime config.toml.
set -e

escape_toml() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

if [ -n "$PORT" ] && [ -z "$WHATOMATE_SERVER_PORT" ]; then
  WHATOMATE_SERVER_PORT="$PORT"
fi

parse_postgres_url() {
  url="$1"
  url="${url#postgresql://}"
  url="${url#postgres://}"

  rest="${url#*@}"
  hostport="${rest%%/*}"
  db="${rest#*/}"
  db="${db%%\?*}"

  host="${hostport%%:*}"
  port="${hostport#*:}"
  if [ "$port" = "$hostport" ]; then
    port=5432
  fi

  userpass="${url%@*}"
  user="${userpass%%:*}"
  pass="${userpass#*:}"

  WHATOMATE_DATABASE_HOST="${WHATOMATE_DATABASE_HOST:-$host}"
  WHATOMATE_DATABASE_PORT="${WHATOMATE_DATABASE_PORT:-$port}"
  WHATOMATE_DATABASE_USER="${WHATOMATE_DATABASE_USER:-$user}"
  WHATOMATE_DATABASE_PASSWORD="${WHATOMATE_DATABASE_PASSWORD:-$pass}"
  WHATOMATE_DATABASE_NAME="${WHATOMATE_DATABASE_NAME:-$db}"
}

parse_redis_url() {
  url="$1"
  WHATOMATE_REDIS_TLS="false"

  case "$url" in
    rediss://*)
      WHATOMATE_REDIS_TLS="true"
      url="${url#rediss://}"
      ;;
    redis://*)
      url="${url#redis://}"
      ;;
    *)
      return 0
      ;;
  esac

  db=0
  case "$url" in
    */*)
      db="${url#*/}"
      db="${db%%\?*}"
      url="${url%%/*}"
      ;;
  esac

  hostport="$url"
  WHATOMATE_REDIS_USERNAME=""
  WHATOMATE_REDIS_PASSWORD=""

  case "$url" in
    *@*)
      hostport="${url#*@}"
      auth="${url%@*}"
      user="${auth%%:*}"
      pass="${auth#*:}"
      if [ "$pass" != "$auth" ]; then
        WHATOMATE_REDIS_USERNAME="$user"
        WHATOMATE_REDIS_PASSWORD="$pass"
      elif [ -n "$auth" ]; then
        WHATOMATE_REDIS_PASSWORD="$auth"
      fi
      ;;
  esac

  host="${hostport%%:*}"
  port="${hostport#*:}"
  if [ "$port" = "$hostport" ]; then
    port=6379
  fi

  WHATOMATE_REDIS_HOST="${WHATOMATE_REDIS_HOST:-$host}"
  WHATOMATE_REDIS_PORT="${WHATOMATE_REDIS_PORT:-$port}"
  WHATOMATE_REDIS_DB="${WHATOMATE_REDIS_DB:-$db}"
}

if [ -n "$DATABASE_URL" ] && [ -z "$WHATOMATE_DATABASE_HOST" ]; then
  parse_postgres_url "$DATABASE_URL"
fi

if [ -n "$REDIS_URL" ] && [ -z "$WHATOMATE_REDIS_HOST" ]; then
  parse_redis_url "$REDIS_URL"
fi

: "${WHATOMATE_SERVER_PORT:=8080}"
: "${WHATOMATE_APP_ENVIRONMENT:=production}"
: "${WHATOMATE_APP_DEBUG:=false}"
: "${WHATOMATE_DATABASE_SSL_MODE:=require}"
: "${WHATOMATE_STORAGE_TYPE:=local}"
: "${WHATOMATE_STORAGE_LOCAL_PATH:=/app/uploads}"
: "${WHATOMATE_RATE_LIMIT_ENABLED:=true}"
: "${WHATOMATE_RATE_LIMIT_TRUST_PROXY:=true}"

if [ -z "$WHATOMATE_DATABASE_HOST" ] || [ -z "$WHATOMATE_REDIS_HOST" ]; then
  echo "render-start.sh: DATABASE_URL and REDIS_URL (or WHATOMATE_DATABASE_HOST / WHATOMATE_REDIS_HOST) are required" >&2
  exit 1
fi

if [ -z "$WHATOMATE_APP_ENCRYPTION_KEY" ] || [ -z "$WHATOMATE_JWT_SECRET" ]; then
  echo "render-start.sh: WHATOMATE_APP_ENCRYPTION_KEY and WHATOMATE_JWT_SECRET must be set" >&2
  exit 1
fi

db_host=$(escape_toml "$WHATOMATE_DATABASE_HOST")
db_user=$(escape_toml "$WHATOMATE_DATABASE_USER")
db_pass=$(escape_toml "$WHATOMATE_DATABASE_PASSWORD")
db_name=$(escape_toml "$WHATOMATE_DATABASE_NAME")
redis_host=$(escape_toml "$WHATOMATE_REDIS_HOST")
redis_user=$(escape_toml "$WHATOMATE_REDIS_USERNAME")
redis_pass=$(escape_toml "$WHATOMATE_REDIS_PASSWORD")
app_key=$(escape_toml "$WHATOMATE_APP_ENCRYPTION_KEY")
jwt_secret=$(escape_toml "$WHATOMATE_JWT_SECRET")

cat > /app/config.render.toml <<EOF
[app]
environment = "${WHATOMATE_APP_ENVIRONMENT}"
debug = ${WHATOMATE_APP_DEBUG}
encryption_key = "${app_key}"

[server]
host = "0.0.0.0"
port = ${WHATOMATE_SERVER_PORT}

[database]
host = "${db_host}"
port = ${WHATOMATE_DATABASE_PORT}
user = "${db_user}"
password = "${db_pass}"
name = "${db_name}"
ssl_mode = "${WHATOMATE_DATABASE_SSL_MODE}"

[redis]
host = "${redis_host}"
port = ${WHATOMATE_REDIS_PORT}
username = "${redis_user}"
password = "${redis_pass}"
db = ${WHATOMATE_REDIS_DB}
tls = ${WHATOMATE_REDIS_TLS}

[jwt]
secret = "${jwt_secret}"

[storage]
type = "${WHATOMATE_STORAGE_TYPE}"
local_path = "${WHATOMATE_STORAGE_LOCAL_PATH}"

[rate_limit]
enabled = ${WHATOMATE_RATE_LIMIT_ENABLED}
trust_proxy = ${WHATOMATE_RATE_LIMIT_TRUST_PROXY}
EOF

if [ -n "$WHATOMATE_WHATSAPP_WEBHOOK_VERIFY_TOKEN" ] \
  || [ -n "$WHATOMATE_WHATSAPP_META_APP_ID" ] \
  || [ -n "$WHATOMATE_WHATSAPP_META_APP_SECRET" ] \
  || [ -n "$WHATOMATE_WHATSAPP_EMBEDDED_SIGNUP_CONFIG_ID" ]; then
  wa_verify=$(escape_toml "$WHATOMATE_WHATSAPP_WEBHOOK_VERIFY_TOKEN")
  wa_app_id=$(escape_toml "$WHATOMATE_WHATSAPP_META_APP_ID")
  wa_app_secret=$(escape_toml "$WHATOMATE_WHATSAPP_META_APP_SECRET")
  wa_config_id=$(escape_toml "$WHATOMATE_WHATSAPP_EMBEDDED_SIGNUP_CONFIG_ID")
  cat >> /app/config.render.toml <<EOF

[whatsapp]
webhook_verify_token = "${wa_verify}"
meta_app_id = "${wa_app_id}"
meta_app_secret = "${wa_app_secret}"
embedded_signup_config_id = "${wa_config_id}"
EOF
fi

set -- "$@" "-config" "/app/config.render.toml"
exec ./whatomate "$@"
