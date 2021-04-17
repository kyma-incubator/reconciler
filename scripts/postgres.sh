#!/bin/bash

CONTAINER_NAME=postgres
POSTGRES_DATA_DIR=$(pwd)/tmp/postgres
POSTGRES_PORT=5432

function containerId() {
  docker ps --filter "name=$CONTAINER_NAME" --format "{{.ID}}"
}

function error() {
  >&2 echo "$1"
}

function start() { 
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
    -e POSTGRES_PASSWORD=kyma \
    -e POSTGRES_USER=kyma \
    -e POSTGRES_DB=kyma \
    postgres)
  exitCode=$?

  if [ $exitCode -eq 0 ]; then
    echo "Postgres container started (listening on port $POSTGRES_PORT)"
  else
    error "Failed to start Postgres container (code: $exitCode)"
    exit $exitCode
  fi
}

function cleanup() {
  docker container prune --force --filter "label=name=$CONTAINER_NAME" > /dev/null
}

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
    exit 0
  else
    error "Failed to stop Postgres container (code: $exitCode, containerId: $id)"
  fi
}

function reset() {
  stop
  rm -rf "$POSTGRES_DATA_DIR"
  mkdir -p "$POSTGRES_DATA_DIR"
  start
}

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
  *)
    echo "
Command '$1' not supported. Please use:

  $> $(basename $0) start|stop|reset|status

  * start   = Start Postgres
  * stop    = Stop Postgres
  * reset   = Erase data and restart Postgres
  * status  = Check if Postgres is running
  * cleanup = Prune remaining postgres container
"
esac

