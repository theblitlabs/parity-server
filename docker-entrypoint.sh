#!/bin/sh
set -e

# Wait for PostgreSQL to be ready
until PGPASSWORD=$DATABASE_PASSWORD psql -h "$DATABASE_HOST" -U "$DATABASE_USERNAME" -d "$DATABASE_DATABASE_NAME" -c '\q'; do
  >&2 echo "Postgres is unavailable - sleeping"
  sleep 1
done

>&2 echo "Postgres is up - executing command"

# Run authentication if PRIVATE_KEY is provided
if [ ! -z "$PRIVATE_KEY" ]; then
  echo "Running authentication..."
  ./parity-server auth --private-key "$PRIVATE_KEY"
fi

# Start the server
echo "Starting server..."
./parity-server server