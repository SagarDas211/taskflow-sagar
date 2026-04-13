#!/bin/sh
set -eu

echo "Running database migrations..."
migrate -database-url "${DATABASE_URL}" -path /app/db/migrations -direction up

echo "Starting API server..."
exec api
