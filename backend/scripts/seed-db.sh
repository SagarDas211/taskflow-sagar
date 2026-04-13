#!/bin/sh
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
BACKEND_DIR="$REPO_ROOT/backend"

if [ -f "$REPO_ROOT/.env" ]; then
  set -a
  # shellcheck disable=SC1091
  . "$REPO_ROOT/.env"
  set +a
fi

POSTGRES_USER=${POSTGRES_USER:-taskflow}
POSTGRES_DB=${POSTGRES_DB:-taskflow}

cd "$REPO_ROOT"

docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" < "$BACKEND_DIR/db/seeds/seed.sql"

echo "Database seeded successfully."
echo "Email: test@example.com"
echo "Password: password123"
