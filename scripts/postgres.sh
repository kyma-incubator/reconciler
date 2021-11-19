#!/bin/bash

# Get current working directory
cd "$(dirname $0)" > /dev/null || return
readonly CWD=$(pwd)
cd - > /dev/null || return

# Script configuration
readonly CONTAINER_NAME="postgres"
readonly POSTGRES_DATA_DIR="${CWD}/tmp/postgres"
readonly POSTGRES_PORT=5432
readonly POSTGRES_USER="kyma"
readonly POSTGRES_PASSWORD="kyma"
readonly POSTGRES_DB="kyma"
readonly POSTGRES_START_DELAY=3
readonly MIGRATE_PATH="${CWD}/../configs/db/postgres"

# Get Postgres container ID
function containerId() {
  docker ps --filter "name=$CONTAINER_NAME" --format "{{.ID}}"
}

# Print to STDERR and exit
function error() {
  local msg="$1"
  local exitCode="$2"

  >&2 echo "$msg"

  if [ -z "$exitCode" ]; then
    exitCode="1"
  fi
  exit $exitCode
}

# Start Postgres container
function start() { 
  local waitForPg=$1
  local id
  local exitCode

  if [ -n "$(containerId)" ]; then
    echo "Postgres container already running"
    exit 0
  fi

  mkdir -p "$POSTGRES_DATA_DIR"

  cleanup #prune any old Postgres container

  id=$(docker run \
    -d \
    -p $POSTGRES_PORT:$POSTGRES_PORT \
    --name "$CONTAINER_NAME" \
    -l "name=$CONTAINER_NAME" \
    -v "$POSTGRES_DATA_DIR":/var/lib/postgresql/data \
    -e POSTGRES_PASSWORD=$POSTGRES_PASSWORD \
    -e POSTGRES_USER=$POSTGRES_USER \
    -e POSTGRES_DB=$POSTGRES_DB \
    postgres)$
  exitCode=$?

  if [ $exitCode -eq 0 ]; then
    echo "Postgres container started (listening on port $POSTGRES_PORT)"
    echo "Waiting for Postgres to be ready"
    if [ -z "$waitForPg" ]; then
      sleep $POSTGRES_START_DELAY
    else
      sleep "$waitForPg"
    fi
    
    migrate
  else
    error "Failed to start Postgres container (code: $exitCode)" $exitCode
  fi
}

# Migrate database schema
function migrate() {
  local postgresDSN="postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@localhost:$POSTGRES_PORT/$POSTGRES_DB?sslmode=disable"
  migrateCmd=$(which migrate)
  if [ $? -ne 0 ]; then
    error "DB migration requires the 'migrate' tool, please install it and try again.
    See https://github.com/golang-migrate/migrate/tree/master/cmd/migrate"
    exit 1
  fi

  pg_isready_cmd=$(which pg_isready)
  if [ $? -ne 0 ]; then
    error "DB migration requires the 'pg_isready' tool, please install postgresql tools and try again.
    See https://www.postgresql.org/docs/current/app-pg-isready.html"
    exit 1
  fi

  echo "Wait for Postgresql to be ready"
  $pg_isready_cmd --host "localhost" --port "${POSTGRES_PORT}" --dbname "${POSTGRES_DB}" --username "${POSTGRES_USER}" --timeout 30
  if [ $? -ne 0 ]; then
    echo "database is not ready, run again migrate"
    exit 1
  fi

  echo -n "Migrating database: "
  $migrateCmd -database "$postgresDSN" -path "$MIGRATE_PATH" up

#  fi
}

# Purge any old Posgres container
function cleanup() {
  docker container prune --force --filter "label=name=$CONTAINER_NAME" > /dev/null
}

# Stop Postgres container
function stop() {
  local id
  local exitCode
  local contId

  contId=$(containerId)
  if [ -z "$contId" ]; then
    echo "Postgres container not running"
    return
  fi

  id=$(docker container kill "$contId")
  exitCode=$? 

  if [ $exitCode -eq 0 ]; then
    cleanup
    echo "Postgres container stopped"
  else
    error "Failed to stop Postgres container (code: $exitCode, containerId: $id)"
  fi
}

# Delete Postgres data directory
function reset() {
  stop
  echo "Cleaning the database"
  rm -rf "$POSTGRES_DATA_DIR"
  mkdir -p "$POSTGRES_DATA_DIR"
  echo "Database clean up finished"
  start 30 #start with a waitFor period of 30 sec (DB-creation on FS can take longer for the first time)
}

# Check whether Postgres container is running
function status() {
  if [ -n "$(containerId)" ]; then
    echo "Postgres is running"
    exit 0
  else
    echo "Postgres not running"
    exit 2
  fi
}

# Dispatch input command
case "$1" in
  start)
    start
    ;;
  stop)
    stop
    ;;
  reset)
    reset
    ;;
  status)
    status
    ;;
  cleanup)
    cleanup
    ;;
  migrate)
    migrate
    ;;
  *)
    echo "
Command '$1' not supported. Please use:

  $> $(basename $0) start|stop|reset|status|cleanup|migrate

  * start   = Start Postgres
  * stop    = Stop Postgres
  * reset   = Erase data and restart Postgres
  * status  = Check if Postgres is running
  * cleanup = Prune remaining postgres container
  * migrate = Migrate DB to latest schema
"
esac

